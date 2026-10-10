import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

/// Catch-up synchronization pipeline for offline-to-online transitions (UST-1.4.3).
///
/// - When a client disconnects or is suspended, the 50-message WebSocket history
///   burst may not bridge the gap. This service fills the gap between the newest
///   locally cached chat message and the live timeline via
///   `GET /api/messages/sync?after={timestamp}`.
/// - **Cursor snapshot**: the cursor is captured *before* (re)connecting, i.e.
///   before the history burst lands in the same cache. Reading it afterwards
///   would point at the newest burst message and skip the gap entirely.
/// - **Paging**: the server returns at most [pageSize] messages per request; the
///   service keeps requesting from the last returned timestamp until a short
///   page arrives.
/// - Timestamps are sent as UTC ISO 8601 (`toUtc().toIso8601String()`).
class SyncService {
  static const int pageSize = 500;
  static const Duration requestTimeout = Duration(seconds: 15);
  static const Duration retryDelay = Duration(seconds: 15);

  final ChatWebSocketService _socketService;
  final LocalMessageRepository _localRepo;
  final http.Client _httpClient;
  final bool _ownsClient;
  final String? _baseUrlOverride;
  String? _accessToken;

  /// Supplies the current access token; takes precedence over [setAccessToken].
  String? Function()? tokenProvider;

  StreamSubscription<SocketStatus>? _statusSubscription;
  SocketStatus _previousStatus = SocketStatus.disconnected;
  bool _isSyncing = false;
  bool get isSyncing => _isSyncing;

  DateTime? _pendingCursor;
  String? _pendingCursorId;
  Future<void>? _cursorCapture;
  Timer? _retryTimer;

  final StreamController<List<ChatMessage>> _syncCompletedController =
      StreamController<List<ChatMessage>>.broadcast();

  /// Reactive stream delivering batches of synchronized catch-up messages.
  Stream<List<ChatMessage>> get onSyncCompleted =>
      _syncCompletedController.stream;

  /// Optional callback invoked when sync completes.
  void Function(List<ChatMessage> messages)? onSyncCallback;

  SyncService({
    required ChatWebSocketService socketService,
    required LocalMessageRepository localRepo,
    http.Client? httpClient,
    String? baseUrlOverride,
    String? accessToken,
    this.onSyncCallback,
    this.tokenProvider,
  }) : _socketService = socketService,
       _localRepo = localRepo,
       _httpClient = httpClient ?? http.Client(),
       _ownsClient = httpClient == null,
       _baseUrlOverride = baseUrlOverride,
       _accessToken = accessToken {
    _startListening();
  }

  /// Updates the authentication token used for catch-up synchronization.
  void setAccessToken(String? token) {
    _accessToken = token;
  }

  /// Records the newest cached chat message as the gap start. Call before
  /// connecting; also done automatically whenever the socket drops.
  Future<void> captureCursor() {
    return _cursorCapture = () async {
      final newest = await _localRepo.getNewestMessage();
      if (newest != null) {
        _pendingCursor = newest.createdAt;
        _pendingCursorId = newest.id;
      }
    }();
  }

  void _startListening() {
    _statusSubscription?.cancel();
    _statusSubscription = _socketService.statusStream.listen((currentStatus) {
      final wasConnected = _previousStatus == SocketStatus.connected;
      final isNowConnected = currentStatus == SocketStatus.connected;
      _previousStatus = currentStatus;

      if (wasConnected && !isNowConnected) {
        // Leaving the live stream: remember where the gap starts.
        unawaited(captureCursor());
      }
      if (!wasConnected && isNowConnected) {
        debugPrint(
          'SyncService: WebSocket connected. Triggering catch-up sync...',
        );
        unawaited(_syncFromPendingCursor());
      }
    });
  }

  Future<void> _syncFromPendingCursor() async {
    await _cursorCapture;
    final cursor = _pendingCursor;
    final result = await _run(after: cursor, afterId: _pendingCursorId);
    if (result != null) {
      _pendingCursor = null;
      _pendingCursorId = null;
    } else {
      _scheduleRetry();
    }
  }

  void _scheduleRetry() {
    _retryTimer?.cancel();
    _retryTimer = Timer(retryDelay, () {
      if (_socketService.status == SocketStatus.connected) {
        unawaited(_syncFromPendingCursor());
      }
    });
  }

  /// Executes the catch-up synchronization request(s).
  ///
  /// Uses [after] if given, otherwise the newest cached chat message. Returns the
  /// messages delivered (empty on failure or when there is nothing to sync).
  Future<List<ChatMessage>> sync({
    DateTime? after,
    String? afterId,
    String? token,
  }) async {
    return await _run(after: after, afterId: afterId, token: token) ?? const [];
  }

  /// Returns null on failure so callers can retry.
  Future<List<ChatMessage>?> _run({
    DateTime? after,
    String? afterId,
    String? token,
  }) async {
    if (_isSyncing) {
      debugPrint(
        'SyncService: Sync already in progress, skipping duplicate call.',
      );
      return null;
    }

    _isSyncing = true;
    try {
      DateTime? cursor = after;
      String? cursorId = afterId;
      if (cursor == null) {
        final newest = await _localRepo.getNewestMessage();
        cursor = newest?.createdAt;
        cursorId = newest?.id;
      }

      // No cached messages: the WebSocket 50-message burst handles fresh hydration.
      if (cursor == null) {
        debugPrint(
          'SyncService: No existing cached messages found. Skipping REST sync (WebSocket hydration handles fresh load).',
        );
        return const [];
      }

      final effectiveToken = token ?? tokenProvider?.call() ?? _accessToken;
      final all = <ChatMessage>[];

      while (true) {
        final batch = await _fetchPage(cursor!, cursorId, effectiveToken);
        if (batch == null) {
          // Keep what we got, but report failure so the gap is retried.
          _emit(all);
          return null;
        }
        all.addAll(batch);
        if (batch.length < pageSize) break;
        cursor = batch.last.createdAt;
        cursorId = batch.last.id;
      }

      debugPrint(
        'SyncService: Catch-up sync delivered ${all.length} missed messages.',
      );
      return _emit(all);
    } catch (e, stack) {
      debugPrint('SyncService: Error executing catch-up sync: $e\n$stack');
      return null;
    } finally {
      _isSyncing = false;
    }
  }

  List<ChatMessage> _emit(List<ChatMessage> messages) {
    if (messages.isNotEmpty && !_syncCompletedController.isClosed) {
      _syncCompletedController.add(messages);
      onSyncCallback?.call(messages);
    }
    return messages;
  }

  Future<List<ChatMessage>?> _fetchPage(
    DateTime after,
    String? afterId,
    String? token,
  ) async {
    final uri = Uri.parse(AppConfig.syncUrl(baseUrlOverride: _baseUrlOverride))
        .replace(
          queryParameters: {
            'after': after.toUtc().toIso8601String(),
            if (afterId != null && afterId.isNotEmpty) 'after_id': afterId,
          },
        );

    final headers = <String, String>{'Accept': 'application/json'};
    if (token != null && token.isNotEmpty) {
      headers['Authorization'] = 'Bearer $token';
    }

    debugPrint('SyncService: Querying catch-up sync: $uri');
    final response = await _httpClient
        .get(uri, headers: headers)
        .timeout(requestTimeout);

    if (response.statusCode != 200) {
      debugPrint(
        'SyncService: Sync endpoint responded with ${response.statusCode}',
      );
      return null;
    }
    return parseMessages(jsonDecode(response.body));
  }

  /// Accepts either a bare JSON array or `{"messages": [...]}`.
  static List<ChatMessage> parseMessages(Object? decoded) {
    final List<Object?> items;
    if (decoded is List) {
      items = decoded;
    } else if (decoded is Map && decoded['messages'] is List) {
      items = decoded['messages'] as List<Object?>;
    } else {
      items = const [];
    }
    return [
      for (final item in items)
        if (item is Map) ChatMessage.fromJson(Map<String, dynamic>.from(item)),
    ];
  }

  /// Disposes active listeners and streams.
  void dispose() {
    _statusSubscription?.cancel();
    _statusSubscription = null;
    _retryTimer?.cancel();
    _syncCompletedController.close();
    if (_ownsClient) {
      _httpClient.close();
    }
  }
}
