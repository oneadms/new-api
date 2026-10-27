// new-api / QuantumNous — derive acceptance from the actual deployment settings.
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export function createAcceptanceJob(deployment) {
  if (deployment.apiVersion !== 'apps/v1' || deployment.kind !== 'Deployment' ||
      deployment.metadata?.name !== 'quality-renderer' || !deployment.metadata.namespace) {
    throw new Error('Expected the namespaced quality-renderer Deployment');
  }
  const pod = structuredClone(deployment.spec.template.spec);
  const worker = pod.containers.find((container) => container.name === 'worker');
  if (!worker || worker.envFrom?.length || worker.env?.some((entry) => entry.name === 'QUALITY_RENDERER_TOKEN')) {
    throw new Error('Acceptance requires a worker without ingress credentials or envFrom');
  }
  worker.command = ['/bin/sh', '/app/src/kubernetes-worker.sh', 'node', '--test', '/app/integration/browser.test.mjs'];
  worker.env = [...(worker.env ?? []), { name: 'QUALITY_RENDERER_INTEGRATION', value: '1' }];
  delete worker.startupProbe;
  delete worker.readinessProbe;
  delete worker.livenessProbe;
  pod.containers = [worker];
  pod.restartPolicy = 'Never';
  return {
    apiVersion: 'batch/v1',
    kind: 'Job',
    metadata: { generateName: 'quality-renderer-acceptance-', namespace: deployment.metadata.namespace },
    spec: {
      backoffLimit: 0,
      activeDeadlineSeconds: 120,
      ttlSecondsAfterFinished: 86400,
      template: { metadata: { labels: { app: 'quality-renderer-acceptance' } }, spec: pod },
    },
  };
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  try {
    let input = '';
    for await (const chunk of process.stdin) input += chunk;
    process.stdout.write(`${JSON.stringify(createAcceptanceJob(JSON.parse(input)), null, 2)}\n`);
  } catch (error) {
    process.stderr.write(`create-acceptance-job: ${error.message}\n`);
    process.exitCode = 1;
  }
}
