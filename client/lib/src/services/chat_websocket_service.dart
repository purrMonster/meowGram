import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:web_socket_channel/status.dart' as ws_status;
import 'package:web_socket_channel/web_socket_channel.dart';

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

/// Service managing the realtime WebSocket channel with Bearer token authentication.
class ChatWebSocketService extends ChangeNotifier {
  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;

  SocketStatus _status = SocketStatus.disconnected;
  SocketStatus get status => _status;

  final List<ChatMessage> _messages = [];
  List<ChatMessage> get messages => List.unmodifiable(_messages);

  String? _lastError;
  String? get lastError => _lastError;

  String? _connectedUrl;
  String? get connectedUrl => _connectedUrl;

  /// Connects to the backend WebSocket endpoint.
  ///
  /// In compliance with Epic 1.2, [accessToken] is injected via the `?token=` query parameter
  /// because browser WebSockets cannot attach custom HTTP Authorization headers during handshake.
  void connect({String? customWsUrl, String? accessToken}) {
    if (_status == SocketStatus.connected ||
        _status == SocketStatus.connecting) {
      return;
    }

    _setStatus(SocketStatus.connecting);
    _lastError = null;

    try {
      String targetUrl;
      if (customWsUrl != null && customWsUrl.isNotEmpty) {
        targetUrl = customWsUrl;
        if (accessToken != null &&
            accessToken.isNotEmpty &&
            !targetUrl.contains('token=')) {
          final sep = targetUrl.contains('?') ? '&' : '?';
          targetUrl =
              '$targetUrl${sep}token=${Uri.encodeComponent(accessToken)}';
        }
      } else if (accessToken != null && accessToken.isNotEmpty) {
        targetUrl = AppConfig.authenticatedWsUrl(accessToken);
      } else {
        targetUrl = AppConfig.wsBaseUrl;
      }

      _connectedUrl = targetUrl;
      final uri = Uri.parse(targetUrl);
      final channel = WebSocketChannel.connect(uri);
      _channel = channel;

      // Handle connection handshake resolution and errors gracefully
      channel.ready.then((_) {
        _setStatus(SocketStatus.connected);
      }).catchError((dynamic error) {
        _lastError = error.toString();
        _setStatus(SocketStatus.error);
      });

      _subscription = channel.stream.listen(
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

    _messages.add(
      ChatMessage(text: text, isFromSelf: true, timestamp: DateTime.now()),
    );
    notifyListeners();

    _channel!.sink.add(text);
  }

  void disconnect() {
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close(ws_status.normalClosure);
    _channel = null;
    _connectedUrl = null;
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
