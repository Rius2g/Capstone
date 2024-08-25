// websocket.ts
const socket = new WebSocket('ws://localhost:8080/ws');

// Connection opened
socket.addEventListener('open', function (event) {
  console.log('WebSocket is connected.');
  socket.send('Hello Server!');
});

// Listen for messages
socket.addEventListener('message', function (event) {
  console.log('Message from server:', event.data);
});

// Handle connection close
socket.addEventListener('close', function (event) {
  console.log('WebSocket is closed now.');
});

// Handle connection errors
socket.addEventListener('error', function (error) {
  console.error('WebSocket error:', error);
});

export default socket;
