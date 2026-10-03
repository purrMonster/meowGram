import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:web_socket_channel/status.dart' as ws_status;
import 'package:web_socket_channel/web_socket_channel.dart';

export 'package:meowgram_client/src/models/chat_message.dart';

enum SocketStatus { disconnected, connecting, connected, error }

/// Service managing the realtime WebSocket channel with Bearer token authentication.
///
/// In compliance with Epic 1.2 and Epic 1.3:
/// - Injects [accessToken] into the `?token=` query parameter because browser WebSockets
///   cannot attach custom HTTP `Authorization` headers during handshake.
/// - Parses incoming text frames as JSON envelopes containing `type`, `id`, `sender_id`, `text_content`, etc.
/// - Handles multi-line frame batches flushed by Gorilla WebSocket write pump.
/// - Exposes reactive streams ([messageStream], [statusStream]) for Bloc integration.
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

  final StreamController<ChatMessage> _messageStreamController =
      StreamController<ChatMessage>.broadcast();
  Stream<ChatMessage> get messageStream => _messageStreamController.stream;

  final StreamController<SocketStatus> _statusStreamController =
      StreamController<SocketStatus>.broadcast();
  Stream<SocketStatus> get statusStream => _statusStreamController.stream;

  /// Connects to the backend WebSocket endpoint.
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
          _handleIncomingData(data.toString());
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

  /// Parses incoming WebSocket payload, handling newline-delimited JSON chunks.
  void _handleIncomingData(String rawData) {
    final lines = rawData.split('\n');
    for (final line in lines) {
      final trimmed = line.trim();
      if (trimmed.isEmpty) continue;

      ChatMessage message;
      try {
        final decoded = jsonDecode(trimmed);
        if (decoded is Map) {
          message = ChatMessage.fromJson(Map<String, dynamic>.from(decoded));
        } else {
          message = ChatMessage(
            textContent: trimmed,
            type: 'chat',
            createdAt: DateTime.now(),
          );
        }
      } catch (_) {
        // Fallback for raw text strings
        message = ChatMessage(
          textContent: trimmed,
          type: 'chat',
          createdAt: DateTime.now(),
        );
      }

      _messages.add(message);
      _messageStreamController.add(message);
    }
    notifyListeners();
  }

  /// Sends a chat message to the server encoded as a JSON envelope.
  void sendMessage(String text) {
    final trimmed = text.trim();
    if (trimmed.isEmpty ||
        _status != SocketStatus.connected ||
        _channel == null) {
      return;
    }

    final payload = jsonEncode({'text_content': trimmed});
    _channel!.sink.add(payload);
  }

  /// Closes the active WebSocket connection cleanly.
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
    _statusStreamController.add(newStatus);
    notifyListeners();
  }

  @override
  void dispose() {
    disconnect();
    _messageStreamController.close();
    _statusStreamController.close();
    super.dispose();
  }
}
