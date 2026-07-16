const { Client } = require('ssh2');
const fs = require('fs');

const { VESPER_DEPLOY_HOST, VESPER_DEPLOY_USER, VESPER_DEPLOY_PASSWORD } = process.env;
if (!VESPER_DEPLOY_HOST || !VESPER_DEPLOY_USER || !VESPER_DEPLOY_PASSWORD) {
  throw new Error('Set VESPER_DEPLOY_HOST, VESPER_DEPLOY_USER and VESPER_DEPLOY_PASSWORD.');
}

const conn = new Client();
conn.on('ready', () => {
  console.log('Client :: ready');
  conn.sftp((err, sftp) => {
    if (err) throw err;
    const readStream = fs.createReadStream('index.html');
    const writeStream = sftp.createWriteStream('/var/www/html/index.html');
    
    writeStream.on('close', () => {
      console.log('File transferred successfully');
      conn.end();
    });
    
    readStream.pipe(writeStream);
  });
}).connect({
  host: VESPER_DEPLOY_HOST,
  port: 22,
  username: VESPER_DEPLOY_USER,
  password: VESPER_DEPLOY_PASSWORD
});
