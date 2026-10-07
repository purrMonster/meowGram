import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:meowgram_client/src/models/chat_message.dart';

/// Local OS notifications for incoming chat messages.
///
/// - Only shown while the app is in the background ([appInForeground] is kept
///   up to date by the app's lifecycle observer); in the foreground the message
///   is already on screen.
/// - Supported on Android, iOS and macOS. Elsewhere every call is a no-op.
/// - Callers decide *which* messages notify (ChatBloc: live chat from others).
class NotificationService {
  static final NotificationService _instance = NotificationService._internal();
  factory NotificationService() => _instance;
  NotificationService._internal();

  final FlutterLocalNotificationsPlugin _plugin =
      FlutterLocalNotificationsPlugin();

  bool _isInitialized = false;

  /// Updated from `didChangeAppLifecycleState`.
  bool appInForeground = true;

  static bool get isSupportedPlatform =>
      !kIsWeb &&
      (defaultTargetPlatform == TargetPlatform.android ||
          defaultTargetPlatform == TargetPlatform.iOS ||
          defaultTargetPlatform == TargetPlatform.macOS);

  Future<void> initialize() async {
    if (_isInitialized || !isSupportedPlatform) return;

    const android = AndroidInitializationSettings('@mipmap/ic_launcher');
    const darwin = DarwinInitializationSettings(
      requestAlertPermission: true,
      requestBadgePermission: true,
      requestSoundPermission: true,
    );

    await _plugin.initialize(
      settings: const InitializationSettings(
        android: android,
        iOS: darwin,
        macOS: darwin,
      ),
      onDidReceiveNotificationResponse: (details) {
        // Tapping the notification simply opens the app on the lounge.
      },
    );

    if (defaultTargetPlatform == TargetPlatform.android) {
      // Android 13+ runtime permission (POST_NOTIFICATIONS).
      await _plugin
          .resolvePlatformSpecificImplementation<
              AndroidFlutterLocalNotificationsPlugin>()
          ?.requestNotificationsPermission();
    }

    _isInitialized = true;
  }

  Future<void> showMessageNotification(ChatMessage message) async {
    if (!isSupportedPlatform || appInForeground) return;
    if (!_isInitialized) await initialize();

    const details = NotificationDetails(
      android: AndroidNotificationDetails(
        'meowgram_messages',
        'Chat Messages',
        channelDescription: 'Notifications for incoming chat messages',
        importance: Importance.high,
        priority: Priority.high,
        showWhen: true,
      ),
      iOS: DarwinNotificationDetails(
        presentAlert: true,
        presentBadge: true,
        presentSound: true,
        threadIdentifier: 'lounge',
      ),
      macOS: DarwinNotificationDetails(
        presentAlert: true,
        presentBadge: true,
        presentSound: true,
        threadIdentifier: 'lounge',
      ),
    );

    await _plugin.show(
      id: notificationIdFor(message),
      title: '@${message.username ?? 'someone'} in the lounge',
      body: message.textContent,
      notificationDetails: details,
    );
  }

  /// Stable 31-bit id per message (String.hashCode is not stable across runs).
  static int notificationIdFor(ChatMessage message) {
    final key = message.id ?? message.createdAt.toIso8601String();
    var hash = 0x811c9dc5;
    for (final unit in key.codeUnits) {
      hash = ((hash ^ unit) * 0x01000193) & 0x7fffffff;
    }
    return hash;
  }

  /// Removes all shown notifications (e.g. on logout).
  Future<void> cancelAll() async {
    if (!isSupportedPlatform || !_isInitialized) return;
    await _plugin.cancelAll();
  }
}
