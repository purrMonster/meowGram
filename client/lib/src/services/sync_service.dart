import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

/// Catch-up synchronization pipeline for offline-to-online transitions (UST-1.4.3).
///
/// Problem & Architecture:
/// - When a client disconnects or background suspension occurs, the default 50-message
///   WebSocket history hydration burst might not bridge the full gap if many messages were sent.
/// - This service deterministically fills the gap between the newest local cached message
///   and the current live timeline via a dedicated REST query (`GET /api/messages/sync?after={timestamp}`).
/// - Triggers automatically whenever [ChatWebSocketService] transitions from disconnected/connecting
///   to [SocketStatus.connected].
///
/// Timezone Standardization:
/// - Converts local cache timestamps to UTC using `toUtc().toIso8601String()` (ending in `Z`),
///   e.g. `2026-10-03T13:40:00.123456Z`.
/// - Encodes query parameters cleanly via [Uri.replace(queryParameters: ...)].
class SyncService {
  final ChatWebSocketService _socketService;
  final LocalMessageRepository _localRepo;
  final http.Client _httpClient;
  final bool _ownsClient;
  final String? _baseUrlOverride;
  String? _accessToken;

  StreamSubscription<SocketStatus>? _statusSubscription;
  SocketStatus _previousStatus = SocketStatus.disconnected;
  bool _isSyncing = false;
  bool get isSyncing => _isSyncing;

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
  })  : _socketService = socketService,
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

  /// Subscribes to the WebSocket service status stream to detect reconnection transitions.
  void _startListening() {
    _statusSubscription?.cancel();
    _statusSubscription = _socketService.statusStream.listen((currentStatus) {
      final wasDisconnected = _previousStatus != SocketStatus.connected;
      final isNowConnected = currentStatus == SocketStatus.connected;

      _previousStatus = currentStatus;

      // Trigger catch-up sync whenever transitioning into connected state
      if (wasDisconnected && isNowConnected) {
        debugPrint(
          'SyncService: WebSocket transition to connected detected. Triggering catch-up sync...',
        );
        unawaited(sync());
      }
    });
  }

  /// Executes the catch-up synchronization request.
  ///
  /// - Queries [LocalMessageRepository] for the newest message timestamp.
  /// - Standardizes to ISO 8601 UTC format.
  /// - Calls `GET /api/messages/sync?after={timestamp}`.
  /// - Emits synchronized messages to [onSyncCompleted] and invokes [onSyncCallback].
  Future<List<ChatMessage>> sync({DateTime? after, String? token}) async {
    if (_isSyncing) {
      debugPrint('SyncService: Sync already in progress, skipping duplicate call.');
      return [];
    }

    _isSyncing = true;
    try {
      // 1. Determine newest message timestamp in local storage
      DateTime? syncAfter = after;
      if (syncAfter == null) {
        final newest = await _localRepo.getNewestMessage();
        if (newest != null) {
          syncAfter = newest.createdAt;
        }
      }

      // If no cached message exists, the initial WebSocket 50-message burst handles hydration
      if (syncAfter == null) {
        debugPrint(
          'SyncService: No existing cached messages found. Skipping REST sync (WebSocket hydration handles fresh load).',
        );
        return [];
      }

      // 2. Format ISO 8601 UTC timestamp
      final afterIso = syncAfter.toUtc().toIso8601String();
      final effectiveToken = token ?? _accessToken;
      final baseUrl = _baseUrlOverride ?? AppConfig.apiBaseUrl;

      // Build target URI with URL-safe encoded query parameters
      final uri = Uri.parse('$baseUrl/api/messages/sync').replace(
        queryParameters: {
          'after': afterIso,
        },
      );

      final headers = <String, String>{
        'Accept': 'application/json',
      };
      if (effectiveToken != null && effectiveToken.isNotEmpty) {
        headers['Authorization'] = 'Bearer $effectiveToken';
      }

      debugPrint('SyncService: Querying catch-up sync: $uri');
      final response = await _httpClient.get(uri, headers: headers);

      if (response.statusCode != 200) {
        debugPrint(
          'SyncService: Sync endpoint responded with error: ${response.statusCode} - ${response.body}',
        );
        return [];
      }

      // 3. Parse JSON message list
      final dynamic decoded = jsonDecode(response.body);
      final List<ChatMessage> missedMessages = [];

      if (decoded is List) {
        for (final item in decoded) {
          if (item is Map) {
            missedMessages.add(
              ChatMessage.fromJson(Map<String, dynamic>.from(item)),
            );
          }
        }
      } else if (decoded is Map && decoded['messages'] is List) {
        for (final item in decoded['messages']) {
          if (item is Map) {
            missedMessages.add(
              ChatMessage.fromJson(Map<String, dynamic>.from(item)),
            );
          }
        }
      }

      debugPrint(
        'SyncService: Catch-up sync delivered ${missedMessages.length} missed messages.',
      );

      if (missedMessages.isNotEmpty) {
        _syncCompletedController.add(missedMessages);
        onSyncCallback?.call(missedMessages);
      }

      return missedMessages;
    } catch (e, stack) {
      debugPrint('SyncService: Error executing catch-up sync: $e\n$stack');
      return [];
    } finally {
      _isSyncing = false;
    }
  }

  /// Disposes active listeners and streams.
  void dispose() {
    _statusSubscription?.cancel();
    _statusSubscription = null;
    _syncCompletedController.close();
    if (_ownsClient) {
      _httpClient.close();
    }
  }
}
