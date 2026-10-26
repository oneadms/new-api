import { connect } from 'node:net';
import { SOCKET_PATH } from './policy.mjs';
// Listening starts only after runtime sandbox verification. No render, network
// health check, credentials, or arbitrary document is submitted here.
const socket = connect(SOCKET_PATH);
socket.setTimeout(2_000);
socket.on('connect', () => { socket.destroy(); process.exit(0); });
socket.on('timeout', () => process.exit(1));
socket.on('error', () => process.exit(1));
