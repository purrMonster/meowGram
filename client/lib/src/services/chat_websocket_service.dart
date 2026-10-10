import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:web_socket_channel/status.dart' as ws_status;
import 'package:web_socket_channel/web_socket_channel.dart';

export 'package:meowgram_client/src/models/chat_message.dart';

enum SocketStatus { disconnected, connecting, connected, error }

/// Service managing the realtime WebSocket channel with Bearer token authentication.
///
/// - Exchanges the access token for a short-lived, one-use ticket before opening
///   the socket. Only the ticket appears in the handshake URL.
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

  /// Endpoint URL for display, with authentication query parameters removed.
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
    unawaited(_open());
  }

  /// Forces a fresh connection (e.g. after a token refresh or app resume).
  void reconnectNow() {
    if (!_autoReconnect) return;
    _reconnectTimer?.cancel();
    _reconnectAttempt = 0;
    if (_status != SocketStatus.connected) {
      unawaited(_open());
    }
  }

  String? _currentToken() {
    final provided = tokenProvider?.call();
    if (provided != null && provided.isNotEmpty) return provided;
    return _explicitToken;
  }

  /// Removes credentials from the URL so it is safe to display or log.
  static String redactToken(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null ||
        (!uri.queryParameters.containsKey('token') &&
            !uri.queryParameters.containsKey('ticket'))) return url;
    final params = Map<String, String>.from(uri.queryParameters)
      ..remove('token')
      ..remove('ticket');
    // Rebuild instead of Uri.replace: replace(queryParameters: null) keeps the
    // original query (and with it the ticket).
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

  Future<void> _open() async {
    _closeChannel();
    final generation = ++_generation;
    _setStatus(SocketStatus.connecting);
    _lastError = null;

    try {
      final token = _currentToken();
      if (token == null || token.isEmpty) {
        throw StateError('A signed-in session is required to connect.');
      }
      final ticket = await _createTicket(token);
      if (generation != _generation) return;
      final base = _customWsUrl?.isNotEmpty == true
          ? Uri.parse(_customWsUrl!)
          : Uri.parse(AppConfig.wsBaseUrl);
      final query = Map<String, String>.from(base.queryParameters)
        ..remove('token')
        ..['ticket'] = ticket;
      final targetUrl = base.replace(queryParameters: query).toString();

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
      if (generation != _generation) return;
      _lastError = e.toString();
      _setStatus(SocketStatus.error);
      _scheduleReconnect();
    }
  }

  Future<String> _createTicket(String accessToken) async {
    final client = http.Client();
    try {
      final response = await client
          .post(
            Uri.parse(AppConfig.wsTicketUrl),
            headers: {
              'Accept': 'application/json',
              'Authorization': 'Bearer $accessToken',
            },
          )
          .timeout(const Duration(seconds: 10));
      if (response.statusCode != 200) {
        throw StateError('WebSocket ticket request failed (${response.statusCode}).');
      }
      final decoded = jsonDecode(response.body);
      if (decoded is! Map ||
          decoded['ticket'] is! String ||
          (decoded['ticket'] as String).isEmpty) {
        throw const FormatException('WebSocket ticket response was invalid.');
      }
      return decoded['ticket'] as String;
    } finally {
      client.close();
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
        if (_autoReconnect && _status != SocketStatus.connected) {
          unawaited(_open());
        }
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
