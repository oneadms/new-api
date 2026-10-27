import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { convertSeccompProfile } from '../kubernetes/generate-seccomp.mjs';

const sourceProfile = JSON.parse(await readFile(new URL('../seccomp_profile.json', import.meta.url), 'utf8'));
const generatorURL = new URL('../kubernetes/generate-seccomp.mjs', import.meta.url);
const target = { architecture: 'amd64', kernelVersion: '6.8.0' };

function profileFixture() {
  return structuredClone(sourceProfile);
}

for (const scenario of [
  { architecture: 'amd64', architectures: ['SCMP_ARCH_X86_64', 'SCMP_ARCH_X86', 'SCMP_ARCH_X32'] },
  { architecture: 'arm64', architectures: ['SCMP_ARCH_AARCH64', 'SCMP_ARCH_ARM'] },
]) {
  test(`${scenario.architecture} expands only its native and source-defined compatibility ABIs`, () => {
    const profile = convertSeccompProfile(sourceProfile, { ...target, architecture: scenario.architecture });
    assert.deepEqual(profile.architectures, scenario.architectures);
    assert.equal(Object.hasOwn(profile, 'archMap'), false);
  });

  test(`${scenario.architecture} keeps default denial, clone3 fallback and only local socket domains`, () => {
    const profile = convertSeccompProfile(sourceProfile, { ...target, architecture: scenario.architecture });
    assert.equal(profile.defaultAction, 'SCMP_ACT_ERRNO');
    assert.equal(profile.defaultErrnoRet, 1);
    assert.deepEqual(profile.syscalls.filter((rule) => rule.names.includes('clone3')), [
      { names: ['clone3'], action: 'SCMP_ACT_ERRNO', errnoRet: 38 },
    ]);
    assert.deepEqual(profile.syscalls.filter((rule) => rule.names.includes('socket')), [
      { names: ['socket'], action: 'SCMP_ACT_ALLOW', args: [{ index: 0, value: 1, op: 'SCMP_CMP_EQ' }] },
      { names: ['socket'], action: 'SCMP_ACT_ALLOW', args: [{ index: 0, value: 16, op: 'SCMP_CMP_EQ' }] },
    ]);
    const allowed = new Set(profile.syscalls.filter((rule) => rule.action === 'SCMP_ACT_ALLOW').flatMap((rule) => rule.names));
    for (const name of ['socketcall', 'io_uring_setup', 'io_uring_enter', 'io_uring_register']) {
      assert.equal(allowed.has(name), false, name);
    }
  });
}

test('native amd64 and arm64 conditions select only the applicable architecture-specific syscalls', () => {
  for (const architecture of ['amd64', 'arm64']) {
    const profile = convertSeccompProfile(sourceProfile, { ...target, architecture });
    const names = new Set(profile.syscalls.flatMap((rule) => rule.names));
    for (const name of ['arch_prctl', 'modify_ldt']) assert.equal(names.has(name), architecture === 'amd64', name);
    for (const name of ['arm_fadvise64_64', 'arm_sync_file_range', 'sync_file_range2', 'breakpoint', 'cacheflush', 'set_tls']) {
      assert.equal(names.has(name), architecture === 'arm64', name);
    }
  }
});

test('compatibility-only includes do not match the native host architecture', () => {
  for (const scenario of [
    { architecture: 'amd64', arches: ['x86', 'x32'] },
    { architecture: 'arm64', arches: ['arm'] },
  ]) {
    const source = profileFixture();
    source.syscalls.push({ names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', includes: { arches: scenario.arches } });
    const profile = convertSeccompProfile(source, { ...target, architecture: scenario.architecture });
    assert.equal(profile.syscalls.some((rule) => rule.names.includes('pidfd_open')), false);
  }
});

test('removing a compatibility ABI in the shared source removes it from generated OCI output', () => {
  const source = profileFixture();
  source.archMap.find((entry) => entry.architecture === 'SCMP_ARCH_X86_64').subArchitectures = [];
  assert.deepEqual(convertSeccompProfile(source, target).architectures, ['SCMP_ARCH_X86_64']);
});

test('ptrace minKernel is inclusive and compares versions numerically rather than lexically', () => {
  for (const [kernelVersion, allowed] of [
    ['3.99.99', false], ['4.7.99', false], ['4.8', true], ['4.8.0', true], ['4.10.0', true], ['5.0.0', true],
  ]) {
    const profile = convertSeccompProfile(sourceProfile, { ...target, kernelVersion });
    assert.equal(profile.syscalls.some((rule) => rule.names.includes('ptrace')), allowed, kernelVersion);
  }
});

test('combined architecture and patch-level kernel conditions require both conditions to match', () => {
  const source = profileFixture();
  source.syscalls.push({ names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', includes: { arches: ['arm64'], minKernel: '6.1.12' } });
  for (const scenario of [
    { architecture: 'amd64', kernelVersion: '6.1.12', allowed: false },
    { architecture: 'arm64', kernelVersion: '6.1.11', allowed: false },
    { architecture: 'arm64', kernelVersion: '6.1.12', allowed: true },
    { architecture: 'arm64', kernelVersion: '6.1.13-1-generic', allowed: true },
    { architecture: 'arm64', kernelVersion: '6.2', allowed: true },
  ]) {
    const profile = convertSeccompProfile(source, { architecture: scenario.architecture, kernelVersion: scenario.kernelVersion });
    assert.equal(profile.syscalls.some((rule) => rule.names.includes('pidfd_open')), scenario.allowed, JSON.stringify(scenario));
  }
});

test('Docker metadata is removed while namespace permissions and OCI argument filters remain intact', () => {
  const source = profileFixture();
  source.comment = 'Docker profile metadata';
  const profile = convertSeccompProfile(source, target);
  assert.deepEqual(Object.keys(profile).sort(), ['architectures', 'defaultAction', 'defaultErrnoRet', 'syscalls']);
  for (const rule of profile.syscalls) {
    assert.ok(Object.keys(rule).every((key) => ['names', 'action', 'args', 'errnoRet'].includes(key)));
  }
  assert.deepEqual(profile.syscalls.find((rule) => rule.names.includes('clone')), {
    names: ['clone', 'setns', 'unshare'], action: 'SCMP_ACT_ALLOW',
  });
  assert.deepEqual(profile.syscalls.find((rule) => rule.names.includes('chroot')), {
    names: ['chroot'], action: 'SCMP_ACT_ALLOW',
  });
  assert.deepEqual(profile.syscalls.find((rule) => rule.names.includes('read')), source.syscalls.find((rule) => rule.names.includes('read')));
  assert.deepEqual(profile.syscalls.filter((rule) => rule.names.includes('personality')), source.syscalls.filter((rule) => rule.names.includes('personality')));
});

test('OCI masked comparisons and explicit denied syscalls survive without widening permissions', () => {
  const source = profileFixture();
  const rule = {
    names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW',
    args: [{ index: 1, value: 3, valueTwo: 1, op: 'SCMP_CMP_MASKED_EQ' }],
  };
  const denied = { names: ['socketcall', 'io_uring_setup'], action: 'SCMP_ACT_ERRNO', errnoRet: 1 };
  source.syscalls.push(rule, denied);
  const profile = convertSeccompProfile(source, target);
  assert.deepEqual(profile.syscalls.find((entry) => entry.names.includes('pidfd_open')), rule);
  assert.deepEqual(profile.syscalls.find((entry) => entry.names.includes('socketcall')), denied);
});

test('conversion is deterministic, does not mutate its source and returns independent nested data', () => {
  const source = profileFixture();
  const before = structuredClone(source);
  const profile = convertSeccompProfile(source, target);
  const expected = structuredClone(profile);
  assert.deepEqual(source, before);
  assert.deepEqual(convertSeccompProfile(source, target), expected);
  profile.architectures.pop();
  profile.syscalls[0].names.push('socketcall');
  profile.syscalls.find((rule) => rule.names.includes('socket')).args[0].value = 2;
  assert.deepEqual(source, before);
  assert.deepEqual(convertSeccompProfile(source, target), expected);
});

test('unsupported or missing target architecture is rejected instead of falling back to the host', () => {
  for (const architecture of [undefined, null, '', 'x86_64', 'aarch64', 'x86', 'arm', 's390x', 'AMD64', 'toString', 64]) {
    assert.throws(() => convertSeccompProfile(sourceProfile, { ...target, architecture }), /target architecture/, String(architecture));
  }
  assert.throws(() => convertSeccompProfile(sourceProfile), /target architecture/);
  assert.throws(() => convertSeccompProfile(sourceProfile, { ...target, platform: 'linux' }), /Unsupported target field/);
});

test('malformed, missing or unsafe target kernel versions are rejected without partial parsing', () => {
  for (const kernelVersion of [
    undefined, null, 6.8, '', '6', '6.1.2.3', '6.1junk', '6.1.0junk', '6.1\n', ' 6.1',
    '0.1', '6.-1', '6.01.0', '6.1-', '6.1--generic', '6.1.0-release/path',
    '9007199254740992.1', '6.9007199254740992', '6.1.9007199254740992',
  ]) {
    assert.throws(() => convertSeccompProfile(sourceProfile, { ...target, kernelVersion }), /Invalid target kernel/, String(kernelVersion));
  }
});

test('unknown or malformed Docker conditions fail closed even on nonmatching architectures', () => {
  for (const includes of [
    null, [], 'arm64', { arches: 'amd64' }, { arches: [] }, { arches: ['amd64', 'unknown'] },
    { caps: ['CAP_SYS_ADMIN'] }, { arches: ['arm64'], caps: [] }, { arches: ['arm64'], maxKernel: '6.8' },
    { arches: ['arm64'], minKernel: '6.8junk' }, { minKernel: 4.8 }, { minKernel: '' },
    { minKernel: '4.8.0-release' }, { minKernel: '6.1.9007199254740992' },
  ]) {
    const source = profileFixture();
    source.syscalls.push({ names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', includes });
    assert.throws(() => convertSeccompProfile(source, target), /includes/, JSON.stringify(includes));
  }
});

test('unsupported profile, rule and argument fields are rejected instead of silently discarded', () => {
  for (const scenario of [
    { mutate: (source) => { source.includes = { arches: ['arm64'] }; }, error: /Unsupported profile field/ },
    { mutate: (source) => { source.syscalls[0].excludes = { arches: ['amd64'] }; }, error: /Unsupported syscalls\[0\] field: excludes/ },
    { mutate: (source) => { source.syscalls[0].incldues = { arches: ['arm64'] }; }, error: /Unsupported syscalls\[0\] field: incldues/ },
    { mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('socket')).args[0].mask = 1; }, error: /Unsupported .*args entry field/ },
  ]) {
    const source = profileFixture();
    scenario.mutate(source);
    assert.throws(() => convertSeccompProfile(source, target), scenario.error);
  }
});

test('missing, ambiguous or unsupported ABI mappings are rejected including unused architecture entries', () => {
  for (const archMap of [
    undefined, [], {},
    [{ architecture: 'SCMP_ARCH_AARCH64', subArchitectures: ['SCMP_ARCH_ARM'] }],
    [...sourceProfile.archMap, sourceProfile.archMap[0]],
    [...sourceProfile.archMap, { architecture: 'SCMP_ARCH_UNKNOWN', subArchitectures: [] }],
    [{ architecture: 'SCMP_ARCH_X86_64', subArchitectures: ['SCMP_ARCH_ARM'] }],
    [{ architecture: 'SCMP_ARCH_X86_64', subArchitectures: ['SCMP_ARCH_X86', 'SCMP_ARCH_X86'] }],
    [{ architecture: 'SCMP_ARCH_X86_64', subArchitectures: null }],
    [sourceProfile.archMap[0], { architecture: 'SCMP_ARCH_AARCH64', subArchitectures: ['SCMP_ARCH_UNKNOWN'] }],
  ]) {
    assert.throws(() => convertSeccompProfile({ ...sourceProfile, archMap }, target), /archMap|subArchitectures/, JSON.stringify(archMap));
  }
});

test('unsupported actions and invalid OCI comparisons are rejected even on filtered-out rules', () => {
  for (const rule of [
    { names: ['pidfd_open'], action: 'SCMP_ACT_LOG' },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ERRNO', errnoRet: 0 },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', args: null },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', args: [{ index: 0, value: 0, op: 'UNKNOWN' }] },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', args: [{ index: 6, value: 0, op: 'SCMP_CMP_EQ' }] },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', args: [{ index: 0, value: Number.MAX_SAFE_INTEGER + 1, op: 'SCMP_CMP_EQ' }] },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', args: [{ index: 0, value: 1, op: 'SCMP_CMP_MASKED_EQ' }] },
    { names: ['pidfd_open'], action: 'SCMP_ACT_ALLOW', args: [{ index: 0, value: 1, valueTwo: -1, op: 'SCMP_CMP_MASKED_EQ' }] },
  ]) {
    const source = profileFixture();
    source.syscalls.push({ ...rule, includes: { arches: ['arm64'] } });
    assert.throws(() => convertSeccompProfile(source, target), /Unsupported|Invalid|must be|requires valueTwo/, JSON.stringify(rule));
  }
});

for (const scenario of [
  { name: 'default allow', mutate: (source) => { source.defaultAction = 'SCMP_ACT_ALLOW'; }, error: /default/ },
  { name: 'default success errno', mutate: (source) => { source.defaultErrnoRet = 0; }, error: /default/ },
  { name: 'allowed clone3', mutate: (source) => { source.syscalls.push({ names: ['clone3'], action: 'SCMP_ACT_ALLOW' }); }, error: /clone3/ },
  { name: 'clone3 EPERM instead of ENOSYS', mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('clone3')).errnoRet = 1; }, error: /clone3/ },
  { name: 'missing clone3 fallback', mutate: (source) => { source.syscalls = source.syscalls.filter((rule) => !rule.names.includes('clone3')); }, error: /clone3/ },
  { name: 'filtered-out clone3 fallback', mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('clone3')).includes = { arches: ['arm64'] }; }, error: /clone3/ },
  { name: 'unfiltered socket', mutate: (source) => { source.syscalls.push({ names: ['socket'], action: 'SCMP_ACT_ALLOW' }); }, error: /socket/ },
  { name: 'AF_INET socket', mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('socket')).args[0].value = 2; }, error: /socket/ },
  { name: 'AF_INET6 socket', mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('socket')).args[0].value = 10; }, error: /socket/ },
  { name: 'non-equality socket domain', mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('socket')).args[0].op = 'SCMP_CMP_NE'; }, error: /socket/ },
  { name: 'wrong socket argument index', mutate: (source) => { source.syscalls.find((rule) => rule.names.includes('socket')).args[0].index = 1; }, error: /socket/ },
  { name: 'missing AF_NETLINK allowance', mutate: (source) => { source.syscalls = source.syscalls.filter((rule) => !rule.names.includes('socket') || rule.args[0].value !== 16); }, error: /AF_NETLINK/ },
]) {
  test(`${scenario.name} in the shared policy fails closed instead of generating an unsafe or incompatible profile`, () => {
    const source = profileFixture();
    scenario.mutate(source);
    assert.throws(() => convertSeccompProfile(source, target), scenario.error);
  });
}

for (const name of ['socketcall', 'io_uring_setup', 'io_uring_enter', 'io_uring_register']) {
  test(`an explicit ${name} allow rule is rejected rather than bypassing socket restrictions`, () => {
    const source = profileFixture();
    source.syscalls.push({ names: [name], action: 'SCMP_ACT_ALLOW' });
    assert.throws(() => convertSeccompProfile(source, target), /socketcall and io_uring must not be allowed/);
  });
}

for (const scenario of [
  { architecture: 'amd64', kernelVersion: '6.8.0-85-generic', args: ['--arch', 'amd64', '--kernel', '6.8.0-85-generic'] },
  { architecture: 'arm64', kernelVersion: '6.6.98+rpt-rpi-v8', args: ['--kernel', '6.6.98+rpt-rpi-v8', '--arch', 'arm64'] },
]) {
  test(`CLI emits only ${scenario.architecture} OCI JSON from the shared source regardless of working directory`, () => {
    const result = spawnSync(process.execPath, [fileURLToPath(generatorURL), ...scenario.args], { cwd: tmpdir(), encoding: 'utf8' });
    assert.equal(result.error, undefined);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stderr, '');
    assert.deepEqual(JSON.parse(result.stdout), convertSeccompProfile(sourceProfile, {
      architecture: scenario.architecture, kernelVersion: scenario.kernelVersion,
    }));
  });
}

test('CLI rejects invalid targets and arguments with nonzero status, stderr diagnostics and empty stdout', () => {
  for (const args of [
    [], ['amd64', '6.8'], ['--arch', 'amd64'],
    ['--arch', 'amd64', '--unknown', '6.8'], ['--arch', 'amd64', '--arch', 'arm64'],
    ['--arch', 'amd64', '--kernel', '6.8', '--extra'],
    ['--arch', 'x86_64', '--kernel', '6.8'], ['--arch', 'amd64', '--kernel', '6.8junk'],
  ]) {
    const result = spawnSync(process.execPath, [fileURLToPath(generatorURL), ...args], { encoding: 'utf8' });
    assert.equal(result.error, undefined);
    assert.equal(result.status, 1, JSON.stringify(args));
    assert.equal(result.stdout, '');
    assert.match(result.stderr, /Usage:|Unsupported target architecture|Invalid target kernel/);
  }
});

test('importing the pure conversion API does not execute the CLI or produce output', () => {
  const result = spawnSync(process.execPath, ['--input-type=module', '--eval', `await import(${JSON.stringify(generatorURL.href)})`], { encoding: 'utf8' });
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, '');
  assert.equal(result.stderr, '');
});
