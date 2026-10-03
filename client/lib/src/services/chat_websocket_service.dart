import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:web_socket_channel/status.dart' as status;

enum SocketStatus { disconnected, connecting, connected, error }

class ChatMessage {
  final String text;
  final bool isFromSelf;
  final DateTime timestamp;

  const ChatMessage({
    required this.text,
    required this.isFromSelf,
    required this.timestamp,
  });
}

class ChatWebSocketService extends ChangeNotifier {
  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;

  SocketStatus _status = SocketStatus.disconnected;
  SocketStatus get status => _status;

  final List<ChatMessage> _messages = [];
  List<ChatMessage> get messages => List.unmodifiable(_messages);

  String? _lastError;
  String? get lastError => _lastError;

  void connect(String wsUrl) {
    if (_status == SocketStatus.connected ||
        _status == SocketStatus.connecting) {
      return;
    }

    _setStatus(SocketStatus.connecting);
    _lastError = null;

    try {
      final uri = Uri.parse(wsUrl);
      _channel = WebSocketChannel.connect(uri);

      _subscription = _channel!.stream.listen(
        (data) {
          if (_status != SocketStatus.connected) {
            _setStatus(SocketStatus.connected);
          }
          final text = data.toString();
          _messages.add(
            ChatMessage(
              text: text,
              isFromSelf: false,
              timestamp: DateTime.now(),
            ),
          );
          notifyListeners();
        },
        onDone: () {
          _setStatus(SocketStatus.disconnected);
        },
        onError: (dynamic error) {
          _lastError = error.toString();
          _setStatus(SocketStatus.error);
        },
        cancelOnError: false,
      );

      // In web/desktop, stream open is ready once listen starts or first message is received
      _setStatus(SocketStatus.connected);
    } catch (e) {
      _lastError = e.toString();
      _setStatus(SocketStatus.error);
    }
  }

  void sendMessage(String text) {
    if (text.trim().isEmpty ||
        _status != SocketStatus.connected ||
        _channel == null) {
      return;
    }

    // Add local sent message to history
    _messages.add(
      ChatMessage(text: text, isFromSelf: true, timestamp: DateTime.now()),
    );
    notifyListeners();

    // Send payload to backend WebSocket endpoint
    _channel!.sink.add(text);
  }

  void disconnect() {
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close(status.normalClosure);
    _channel = null;
    _setStatus(SocketStatus.disconnected);
  }

  void _setStatus(SocketStatus newStatus) {
    _status = newStatus;
    notifyListeners();
  }

  @override
  void dispose() {
    disconnect();
    super.dispose();
  }
}
