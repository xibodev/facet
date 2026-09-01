const http = require('http');
const fs = require('fs');
const path = require('path');

const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  fs.createReadStream(path.join(__dirname, 'design.html')).pipe(res);
});

server.listen(9876, '127.0.0.1', () => {
  console.log('Design server running at http://127.0.0.1:9876/');
});
