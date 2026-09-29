import net from 'node:net';
import { spawn } from 'node:child_process';

// The relay shares Studio's namespace and preserves HTTP Host/Origin and SSE.
const child = spawn('/home/facet/.facet/bin/facet', ['ui', '--port', '8787', '--dir', '/home/facet/studio', '--no-open'], { stdio: 'inherit' });
const relay = net.createServer(client => {
  const upstream = net.connect(8787, '127.0.0.1');
  client.pipe(upstream).pipe(client);
  client.on('error', () => upstream.destroy());
  upstream.on('error', () => client.destroy());
  client.on('close', () => upstream.destroy());
});
relay.listen(8788, '0.0.0.0');
relay.on('error', () => { child.kill('SIGTERM'); process.exitCode = 1; });
child.on('error', () => { relay.close(); process.exitCode = 1; });
child.on('exit', code => { relay.close(); process.exit(code ?? 1); });
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => { relay.close(); child.kill(signal); });
