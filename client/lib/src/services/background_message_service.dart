import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:workmanager/workmanager.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/token_storage.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/services/notification_service.dart';
import 'package:meowgram_client/src/services/sync_service.dart';

const String kBackgroundMessageSyncTask = 'com.purrbrews.meowgram.syncMessages';
const String kBackgroundSyncUniqueName = 'meowgram-sync-periodic';
const String _kCursorKey = 'meowgram_bg_sync_cursor';

/// Periodic background check for new lounge messages (opt-in, Android only).
///
/// Enable with `--dart-define=ENABLE_BACKGROUND_SYNC=true`. Off by default:
/// FCM pushes are the primary notification path.
///
/// Safety rules for the background isolate:
/// - It never opens the Hive message cache (Hive is not multi-isolate safe; the
///   UI isolate owns it). The app's catch-up sync fills the cache on next launch.
/// - It keeps its own cursor in secure storage and only shows a notification
///   ("N new messages") for messages from other people.
/// - It does not refresh tokens; with an expired token it simply skips the run.
///
/// iOS is not supported: it would need BGTaskScheduler identifiers in Info.plist
/// and AppDelegate registration, and Apple discourages polling for chat.
class BackgroundMessageService {
  static const bool enabled = bool.fromEnvironment('ENABLE_BACKGROUND_SYNC');

  static bool get _platformSupported =>
      !kIsWeb && defaultTargetPlatform == TargetPlatform.android;

  /// Registers the periodic task when enabled; otherwise cancels any task left
  /// registered by an earlier build.
  static Future<void> configure() async {
    if (!_platformSupported) return;
    await Workmanager().initialize(callbackDispatcher);
    if (enabled) {
      await Workmanager().registerPeriodicTask(
        kBackgroundSyncUniqueName,
        kBackgroundMessageSyncTask,
        frequency: const Duration(minutes: 15),
        constraints: Constraints(networkType: NetworkType.connected),
      );
    } else {
      await Workmanager().cancelByUniqueName(kBackgroundSyncUniqueName);
    }
  }
}

@pragma('vm:entry-point')
void callbackDispatcher() {
  Workmanager().executeTask((task, inputData) async {
    WidgetsFlutterBinding.ensureInitialized();
    try {
      await runBackgroundCheck();
      return true;
    } catch (e, stack) {
      debugPrint('Background sync error: $e\n$stack');
      return false;
    }
  });
}

/// One background run. Exposed for testing.
Future<void> runBackgroundCheck({
  TokenStorage? tokenStorage,
  FlutterSecureStorage? cursorStorage,
  http.Client? httpClient,
  Future<void> Function(int count, ChatMessage latest)? notify,
}) async {
  final tokens = await (tokenStorage ?? TokenStorage()).readTokens();
  if (tokens == null || tokens.accessToken.isEmpty || tokens.isExpired) {
    return; // Signed out or token expired: nothing we can do in the background.
  }

  final storage = cursorStorage ?? const FlutterSecureStorage();
  final cursorRaw = await storage.read(key: _kCursorKey);
  DateTime? cursor;
  String? cursorId;
  if (cursorRaw != null) {
    try {
      final decoded = jsonDecode(cursorRaw);
      if (decoded is Map) {
        cursor = DateTime.tryParse(decoded['created_at']?.toString() ?? '');
        cursorId = decoded['id']?.toString();
      }
    } catch (_) {
      // Existing installs stored the cursor as an ISO timestamp only.
      cursor = DateTime.tryParse(cursorRaw);
    }
  }
  if (cursor == null) {
    // First run: start watching from now.
    await storage.write(
        key: _kCursorKey, value: DateTime.now().toUtc().toIso8601String());
    return;
  }

  final client = httpClient ?? http.Client();
  try {
    final messages = <ChatMessage>[];
    while (true) {
      final uri = Uri.parse(AppConfig.syncUrl()).replace(
        queryParameters: {
          'after': cursor.toUtc().toIso8601String(),
          if (cursorId != null && cursorId.isNotEmpty) 'after_id': cursorId,
        },
      );
      final response = await client.get(uri, headers: {
        'Accept': 'application/json',
        'Authorization': 'Bearer ${tokens.accessToken}',
      }).timeout(SyncService.requestTimeout);
      if (response.statusCode != 200) {
        debugPrint('Background sync skipped: HTTP ${response.statusCode}');
        return;
      }

      final batch = SyncService.parseMessages(jsonDecode(response.body));
      if (batch.isEmpty) break;
      messages.addAll(batch);
      cursor = batch.last.createdAt;
      cursorId = batch.last.id;
      await storage.write(
        key: _kCursorKey,
        value: jsonEncode({
          'created_at': cursor.toUtc().toIso8601String(),
          'id': cursorId,
        }),
      );
      if (batch.length < SyncService.pageSize) break;
    }

    if (messages.isEmpty) return;

    final mySub = UserProfile.fromTokens(
      accessToken: tokens.accessToken,
      idToken: tokens.idToken,
    ).sub;
    final fromOthers = messages.where((m) => m.senderId != mySub).toList();
    if (fromOthers.isEmpty) return;

    if (notify != null) {
      await notify(fromOthers.length, fromOthers.last);
    } else {
      final notifications = NotificationService()..appInForeground = false;
      final latest = fromOthers.last;
      await notifications.showMessageNotification(
        fromOthers.length == 1
            ? latest
            : ChatMessage(
                id: latest.id,
                username: latest.username,
                textContent: '${fromOthers.length} new messages',
                createdAt: latest.createdAt,
              ),
      );
    }
  } finally {
    if (httpClient == null) client.close();
  }
}
