import assert from 'node:assert/strict';
import test from 'node:test';
import { createAcceptanceJob } from '../kubernetes/create-acceptance-job.mjs';

function deploymentFixture() {
  return {
    apiVersion: 'apps/v1', kind: 'Deployment',
    metadata: { name: 'quality-renderer', namespace: 'production' },
    spec: { template: { spec: {
      automountServiceAccountToken: false,
      nodeSelector: { 'kubernetes.io/hostname': 'renderer-node' },
      volumes: [{ name: 'socket', emptyDir: { medium: 'Memory', sizeLimit: '8Mi' } }],
      containers: [
        { name: 'ingress', env: [{ name: 'QUALITY_RENDERER_TOKEN', valueFrom: { secretKeyRef: { name: 'renderer-auth', key: 'token' } } }] },
        { name: 'worker', image: 'renderer@sha256:pinned',
          securityContext: { allowPrivilegeEscalation: false, seccompProfile: { type: 'Localhost', localhostProfile: 'worker.json' } },
          resources: { limits: { memory: '1Gi' } },
          readinessProbe: { exec: { command: ['node', 'healthcheck.mjs'] } } },
      ],
    } } },
  };
}

test('acceptance uses production worker isolation without service labels or ingress credentials', () => {
  const deployment = deploymentFixture();
  const original = structuredClone(deployment);
  const job = createAcceptanceJob(deployment);
  const pod = job.spec.template.spec;
  assert.equal(job.metadata.namespace, 'production');
  assert.equal(job.spec.template.metadata.labels.app, 'quality-renderer-acceptance');
  assert.equal(pod.containers.length, 1);
  assert.equal(pod.containers[0].name, 'worker');
  assert.equal(pod.containers[0].image, original.spec.template.spec.containers[1].image);
  assert.deepEqual(pod.containers[0].securityContext, original.spec.template.spec.containers[1].securityContext);
  assert.deepEqual(pod.containers[0].resources, original.spec.template.spec.containers[1].resources);
  assert.deepEqual(pod.volumes, original.spec.template.spec.volumes);
  assert.deepEqual(pod.containers[0].env, [{ name: 'QUALITY_RENDERER_INTEGRATION', value: '1' }]);
  assert.equal(pod.automountServiceAccountToken, false);
  assert.equal(pod.restartPolicy, 'Never');
  assert.equal(job.spec.backoffLimit, 0);
  assert.deepEqual(deployment, original);
});

for (const credentials of [
  { env: [{ name: 'QUALITY_RENDERER_TOKEN', value: 'not-a-deployed-token' }] },
  { envFrom: [{ secretRef: { name: 'application-secrets' } }] },
]) {
  test(`worker ${Object.keys(credentials)[0]} credentials are rejected rather than hidden by acceptance`, () => {
    const deployment = deploymentFixture();
    Object.assign(deployment.spec.template.spec.containers[1], credentials);
    assert.throws(() => createAcceptanceJob(deployment), /without ingress credentials or envFrom/);
  });
}

test('an unrelated deployment cannot be used as renderer acceptance input', () => {
  const deployment = deploymentFixture();
  deployment.metadata.name = 'new-api';
  assert.throws(() => createAcceptanceJob(deployment), /quality-renderer Deployment/);
});
