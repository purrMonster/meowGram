import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:web_socket_channel/status.dart' as ws_status;
import 'package:web_socket_channel/web_socket_channel.dart';

export 'package:meowgram_client/src/models/chat_message.dart';

enum SocketStatus { disconnected, connecting, connected, error }

/// Service managing the realtime WebSocket channel with Bearer token authentication.
///
/// - Injects the access token into the `?token=` query parameter because browser
///   WebSockets cannot attach custom HTTP `Authorization` headers during handshake.
///   The token is never exposed through [connectedUrl] (shown in the UI).
/// - Parses incoming text frames as JSON envelopes; handles newline-delimited
///   batches flushed by the Gorilla WebSocket write pump.
/// - Reconnects automatically with exponential backoff (1 s → 30 s) after an
///   unexpected drop, fetching a fresh token from [tokenProvider] on every
///   attempt. [disconnect] stops reconnecting.
class ChatWebSocketService extends ChangeNotifier {
  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;
  Timer? _reconnectTimer;
  int _reconnectAttempt = 0;
  bool _autoReconnect = false;
  int _generation = 0;
  final Random _random = Random();

  /// Supplies the current access token for each (re)connect attempt.
  String? Function()? tokenProvider;

  String? _customWsUrl;
  String? _explicitToken;

  SocketStatus _status = SocketStatus.disconnected;
  SocketStatus get status => _status;

  String? _lastError;
  String? get lastError => _lastError;

  String? _connectedUrl;

  /// Endpoint URL for display, with the token query parameter removed.
  String? get connectedUrl => _connectedUrl;

  final StreamController<ChatMessage> _messageStreamController =
      StreamController<ChatMessage>.broadcast();
  Stream<ChatMessage> get messageStream => _messageStreamController.stream;

  final StreamController<SocketStatus> _statusStreamController =
      StreamController<SocketStatus>.broadcast();
  Stream<SocketStatus> get statusStream => _statusStreamController.stream;

  /// Connects to the backend WebSocket endpoint and keeps reconnecting until
  /// [disconnect] is called.
  void connect({String? customWsUrl, String? accessToken}) {
    _autoReconnect = true;
    _customWsUrl = customWsUrl;
    if (accessToken != null && accessToken.isNotEmpty) {
      _explicitToken = accessToken;
    }
    if (_status == SocketStatus.connected ||
        _status == SocketStatus.connecting) {
      return;
    }
    _reconnectTimer?.cancel();
    _open();
  }

  /// Forces a fresh connection (e.g. after a token refresh or app resume).
  void reconnectNow() {
    if (!_autoReconnect) return;
    _reconnectTimer?.cancel();
    _reconnectAttempt = 0;
    if (_status != SocketStatus.connected) {
      _open();
    }
  }

  String? _currentToken() {
    final provided = tokenProvider?.call();
    if (provided != null && provided.isNotEmpty) return provided;
    return _explicitToken;
  }

  /// Removes the `token` query parameter so the URL is safe to display or log.
  static String redactToken(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null || !uri.queryParameters.containsKey('token')) return url;
    final params = Map<String, String>.from(uri.queryParameters)
      ..remove('token');
    // Rebuild instead of Uri.replace: replace(queryParameters: null) keeps the
    // original query (and with it the token).
    return Uri(
      scheme: uri.scheme,
      userInfo: uri.userInfo.isEmpty ? null : uri.userInfo,
      host: uri.host,
      port: uri.hasPort ? uri.port : null,
      path: uri.path,
      queryParameters: params.isEmpty ? null : params,
      fragment: uri.hasFragment ? uri.fragment : null,
    ).toString();
  }

  void _open() {
    _closeChannel();
    final generation = ++_generation;
    _setStatus(SocketStatus.connecting);
    _lastError = null;

    try {
      final token = _currentToken();
      String targetUrl;
      if (_customWsUrl != null && _customWsUrl!.isNotEmpty) {
        targetUrl = _customWsUrl!;
        if (token != null && token.isNotEmpty && !targetUrl.contains('token=')) {
          final sep = targetUrl.contains('?') ? '&' : '?';
          targetUrl = '$targetUrl${sep}token=${Uri.encodeComponent(token)}';
        }
      } else if (token != null && token.isNotEmpty) {
        targetUrl = AppConfig.authenticatedWsUrl(token);
      } else {
        targetUrl = AppConfig.wsBaseUrl;
      }

      _connectedUrl = redactToken(targetUrl);
      final channel = WebSocketChannel.connect(Uri.parse(targetUrl));
      _channel = channel;

      channel.ready.then((_) {
        if (generation != _generation) return;
        _reconnectAttempt = 0;
        _setStatus(SocketStatus.connected);
      }).catchError((Object error) {
        if (generation != _generation) return;
        _lastError = error.toString();
        _setStatus(SocketStatus.error);
        _scheduleReconnect();
      });

      _subscription = channel.stream.listen(
        (dynamic data) {
          if (generation != _generation) return;
          if (_status != SocketStatus.connected) {
            _reconnectAttempt = 0;
            _setStatus(SocketStatus.connected);
          }
          _handleIncomingData(data.toString());
        },
        onDone: () {
          if (generation != _generation) return;
          _setStatus(SocketStatus.disconnected);
          _scheduleReconnect();
        },
        onError: (Object error) {
          if (generation != _generation) return;
          _lastError = error.toString();
          _setStatus(SocketStatus.error);
        },
        cancelOnError: false,
      );
    } catch (e) {
      _lastError = e.toString();
      _setStatus(SocketStatus.error);
      _scheduleReconnect();
    }
  }

  void _scheduleReconnect() {
    if (!_autoReconnect) return;
    if (_reconnectTimer?.isActive ?? false) return;
    final base = min(30, 1 << min(_reconnectAttempt, 5)); // 1,2,4,8,16,30 s
    final jitterMs = _random.nextInt(500);
    _reconnectAttempt++;
    _reconnectTimer = Timer(
      Duration(seconds: base, milliseconds: jitterMs),
      () {
        if (_autoReconnect && _status != SocketStatus.connected) _open();
      },
    );
  }

  /// Parses incoming WebSocket payload, handling newline-delimited JSON chunks.
  void _handleIncomingData(String rawData) {
    for (final line in rawData.split('\n')) {
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
      _messageStreamController.add(message);
    }
  }

  /// Sends a chat message to the server encoded as a JSON envelope.
  void sendMessage(String text) {
    final trimmed = text.trim();
    if (trimmed.isEmpty ||
        _status != SocketStatus.connected ||
        _channel == null) {
      return;
    }
    _channel!.sink.add(jsonEncode({'text_content': trimmed}));
  }

  void _closeChannel() {
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close(ws_status.normalClosure);
    _channel = null;
  }

  /// Closes the active WebSocket connection cleanly and stops reconnecting.
  void disconnect() {
    _autoReconnect = false;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _generation++;
    _closeChannel();
    _connectedUrl = null;
    _setStatus(SocketStatus.disconnected);
  }

  void _setStatus(SocketStatus newStatus) {
    _status = newStatus;
    if (!_statusStreamController.isClosed) {
      _statusStreamController.add(newStatus);
    }
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
