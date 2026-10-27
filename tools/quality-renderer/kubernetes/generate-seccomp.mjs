// new-api / QuantumNous — convert the shared Docker profile for Kubernetes/containerd.
// Usage: node tools/quality-renderer/kubernetes/generate-seccomp.mjs --arch amd64 --kernel 6.8.0-85-generic
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

// These are ABI identifiers, not a second syscall policy. Enabled ABIs come from archMap.
const ARCHITECTURES = new Map([
  ['amd64', { native: 'SCMP_ARCH_X86_64', compatible: ['SCMP_ARCH_X86', 'SCMP_ARCH_X32'] }],
  ['arm64', { native: 'SCMP_ARCH_AARCH64', compatible: ['SCMP_ARCH_ARM'] }],
]);
const CONDITION_ARCHITECTURES = new Set(['amd64', 'arm64', 'x86', 'x32', 'arm']);
const COMPARISONS = new Set([
  'SCMP_CMP_NE', 'SCMP_CMP_LT', 'SCMP_CMP_LE', 'SCMP_CMP_EQ',
  'SCMP_CMP_GE', 'SCMP_CMP_GT', 'SCMP_CMP_MASKED_EQ',
]);
const USAGE = 'Usage: node tools/quality-renderer/kubernetes/generate-seccomp.mjs --arch <amd64|arm64> --kernel <major.minor[.patch][-release]>';

function validateFields(value, allowed, label) {
  if (value === null || typeof value !== 'object' || Array.isArray(value)
      || ![Object.prototype, null].includes(Object.getPrototypeOf(value))) {
    throw new Error(`${label} must be a JSON object`);
  }
  for (const key of Object.keys(value)) {
    if (!allowed.includes(key)) throw new Error(`Unsupported ${label} field: ${key}`);
  }
  if (Object.hasOwn(value, 'comment') && typeof value.comment !== 'string') {
    throw new Error(`${label}.comment must be a string`);
  }
}

function parseKernelVersion(value, label, allowReleaseSuffix = false) {
  // Only target releases may have a uname -r distribution/local-version suffix.
  const pattern = allowReleaseSuffix
    ? /^([1-9]\d*)\.(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:[-+][A-Za-z0-9][A-Za-z0-9._+-]*)?$/
    : /^([1-9]\d*)\.(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?$/;
  const match = typeof value === 'string' ? pattern.exec(value) : null;
  if (!match || match[0] !== value) throw new Error(`Invalid ${label}: expected a Linux kernel version`);
  const version = [Number(match[1]), Number(match[2]), Number(match[3] ?? 0)];
  if (!version.every(Number.isSafeInteger)) throw new Error(`Invalid ${label}: version component is too large`);
  return version;
}

function expandArchitectures(archMap, architecture) {
  if (!Array.isArray(archMap) || archMap.length === 0) throw new Error('archMap must be a non-empty array');
  const seen = new Set();
  let selected;
  for (const entry of archMap) {
    validateFields(entry, ['architecture', 'subArchitectures'], 'archMap entry');
    const family = [...ARCHITECTURES.values()].find((candidate) => candidate.native === entry.architecture);
    if (!family || seen.has(entry.architecture)) throw new Error('Unsupported or duplicate archMap architecture');
    seen.add(entry.architecture);
    const subArchitectures = Object.hasOwn(entry, 'subArchitectures') ? entry.subArchitectures : [];
    if (!Array.isArray(subArchitectures)
        || subArchitectures.some((sub) => !family.compatible.includes(sub))
        || new Set(subArchitectures).size !== subArchitectures.length) {
      throw new Error(`Invalid subArchitectures for ${entry.architecture}`);
    }
    if (entry.architecture === ARCHITECTURES.get(architecture).native) {
      selected = [entry.architecture, ...subArchitectures];
    }
  }
  if (!selected) throw new Error(`archMap has no entry for ${architecture}`);
  return selected;
}

function matchesIncludes(includes, architecture, kernel, label) {
  validateFields(includes, ['arches', 'minKernel'], label);
  let matches = true;
  if (Object.hasOwn(includes, 'arches')) {
    if (!Array.isArray(includes.arches) || includes.arches.length === 0
        || includes.arches.some((arch) => !CONDITION_ARCHITECTURES.has(arch))) {
      throw new Error(`${label}.arches must be a non-empty array of supported Docker architecture names`);
    }
    // Docker conditions match the native host, not every compatibility ABI in archMap.
    matches = includes.arches.includes(architecture);
  }
  if (Object.hasOwn(includes, 'minKernel')) {
    // Validate even if the architecture does not match: unknown conditions must fail closed.
    const minimum = parseKernelVersion(includes.minKernel, `${label}.minKernel`);
    for (let index = 0; index < kernel.length; index++) {
      if (kernel[index] === minimum[index]) continue;
      matches = matches && kernel[index] > minimum[index];
      break;
    }
  }
  return matches;
}

function assertRendererIsolation(syscalls) {
  let clone3Fallback = false;
  const socketFamilies = new Set();
  for (const rule of syscalls) {
    if (rule.names.includes('clone3')) {
      if (rule.action !== 'SCMP_ACT_ERRNO' || rule.errnoRet !== 38 || rule.args?.length) {
        throw new Error('clone3 must unconditionally return ENOSYS (38)');
      }
      clone3Fallback = true;
    }
    if (rule.action !== 'SCMP_ACT_ALLOW') continue;
    if (rule.names.some((name) => name === 'socketcall' || name.startsWith('io_uring'))) {
      throw new Error('socketcall and io_uring must not be allowed');
    }
    if (rule.names.includes('socket')) {
      const argument = rule.args?.[0];
      if (rule.args?.length !== 1 || argument.index !== 0 || argument.op !== 'SCMP_CMP_EQ'
          || ![1, 16].includes(argument.value)) {
        throw new Error('socket may only allow AF_UNIX (1) or AF_NETLINK (16) by exact domain comparison');
      }
      socketFamilies.add(argument.value);
    }
  }
  if (!clone3Fallback) throw new Error('Missing unconditional clone3 ENOSYS fallback');
  if (!socketFamilies.has(1) || !socketFamilies.has(16)) throw new Error('Missing AF_UNIX or AF_NETLINK socket allowance');
}

/**
 * Pure, fail-closed conversion of the renderer's Docker JSON to an OCI LinuxSeccomp object.
 * Target architecture/kernel are explicit; no host detection, I/O, or source mutation occurs.
 * Only the source profile's supported schema is accepted. In particular, capability filters,
 * excludes and future Docker conditions require deliberate support rather than being dropped.
 */
export function convertSeccompProfile(source, options = {}) {
  validateFields(options, ['architecture', 'kernelVersion'], 'target');
  if (!ARCHITECTURES.has(options.architecture)) throw new Error('Unsupported target architecture: use amd64 or arm64');
  const kernel = parseKernelVersion(options.kernelVersion, 'target kernel version', true);
  validateFields(source, ['defaultAction', 'defaultErrnoRet', 'archMap', 'syscalls', 'comment'], 'profile');
  if (source.defaultAction !== 'SCMP_ACT_ERRNO' || source.defaultErrnoRet !== 1) {
    throw new Error('Renderer profile must default to SCMP_ACT_ERRNO with EPERM (1)');
  }
  const architectures = expandArchitectures(source.archMap, options.architecture);
  if (!Array.isArray(source.syscalls)) throw new Error('syscalls must be an array');
  const syscalls = [];
  for (const [index, rule] of source.syscalls.entries()) {
    const label = `syscalls[${index}]`;
    validateFields(rule, ['names', 'action', 'errnoRet', 'args', 'includes', 'comment'], label);
    if (!Array.isArray(rule.names) || rule.names.length === 0
        || rule.names.some((name) => typeof name !== 'string' || !/^[a-z_][a-z0-9_]*$/.test(name) || name.trim() !== name)) {
      throw new Error(`${label}.names must contain syscall names`);
    }
    if (!['SCMP_ACT_ALLOW', 'SCMP_ACT_ERRNO'].includes(rule.action)) {
      throw new Error(`Unsupported ${label}.action`);
    }
    const converted = { names: [...rule.names], action: rule.action };
    if (Object.hasOwn(rule, 'errnoRet')) {
      if (rule.action !== 'SCMP_ACT_ERRNO' || !Number.isInteger(rule.errnoRet) || rule.errnoRet < 1 || rule.errnoRet > 4095) {
        throw new Error(`Invalid ${label}.errnoRet`);
      }
      converted.errnoRet = rule.errnoRet;
    }
    if (Object.hasOwn(rule, 'args')) {
      if (!Array.isArray(rule.args)) throw new Error(`${label}.args must be an array`);
      converted.args = rule.args.map((argument) => {
        validateFields(argument, ['index', 'value', 'valueTwo', 'op'], `${label}.args entry`);
        if (!Number.isInteger(argument.index) || argument.index < 0 || argument.index > 5
            || !Number.isSafeInteger(argument.value) || argument.value < 0
            || !COMPARISONS.has(argument.op)) {
          throw new Error(`Invalid ${label}.args comparison`);
        }
        if (Object.hasOwn(argument, 'valueTwo')
            && (!Number.isSafeInteger(argument.valueTwo) || argument.valueTwo < 0)) {
          throw new Error(`Invalid ${label}.args valueTwo`);
        }
        if (argument.op === 'SCMP_CMP_MASKED_EQ' && !Object.hasOwn(argument, 'valueTwo')) {
          throw new Error(`${label}.args masked comparison requires valueTwo`);
        }
        return { ...argument };
      });
    }
    if (Object.hasOwn(rule, 'includes')
        && !matchesIncludes(rule.includes, options.architecture, kernel, `${label}.includes`)) continue;
    syscalls.push(converted);
  }
  assertRendererIsolation(syscalls);
  return { defaultAction: source.defaultAction, defaultErrnoRet: source.defaultErrnoRet, architectures, syscalls };
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  try {
    const args = process.argv.slice(2);
    if (args.length !== 4) throw new Error(USAGE);
    const options = new Map();
    for (let index = 0; index < args.length; index += 2) {
      if (!['--arch', '--kernel'].includes(args[index]) || options.has(args[index])) throw new Error(USAGE);
      options.set(args[index], args[index + 1]);
    }
    const source = JSON.parse(await readFile(new URL('../seccomp_profile.json', import.meta.url), 'utf8'));
    const profile = convertSeccompProfile(source, { architecture: options.get('--arch'), kernelVersion: options.get('--kernel') });
    process.stdout.write(`${JSON.stringify(profile, null, 2)}\n`);
  } catch (error) {
    process.stderr.write(`generate-seccomp: ${error.message}\n`);
    process.exitCode = 1;
  }
}
