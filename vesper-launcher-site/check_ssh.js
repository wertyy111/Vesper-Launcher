const { Client } = require('ssh2');

const { VESPER_DEPLOY_HOST, VESPER_DEPLOY_USER, VESPER_DEPLOY_PASSWORD } = process.env;
if (!VESPER_DEPLOY_HOST || !VESPER_DEPLOY_USER || !VESPER_DEPLOY_PASSWORD) {
  throw new Error('Set VESPER_DEPLOY_HOST, VESPER_DEPLOY_USER and VESPER_DEPLOY_PASSWORD.');
}

const conn = new Client();
conn.on('ready', () => {
  conn.exec('cat /etc/nginx/sites-available/default', (err, stream) => {
    if (err) throw err;
    stream.on('close', (code, signal) => {
      conn.end();
    }).on('data', (data) => {
      console.log('STDOUT: ' + data);
    });
  });
}).connect({
  host: VESPER_DEPLOY_HOST,
  port: 22,
  username: VESPER_DEPLOY_USER,
  password: VESPER_DEPLOY_PASSWORD
});
