import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';

/// FCM topic the server publishes lounge-activity pushes to.
const String kLoungeTopic = 'room_lounge';

@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  // Pushes are content-free notification messages; the OS displays them.
  debugPrint('Background push received: ${message.messageId}');
}

/// Push notifications via Firebase Cloud Messaging.
///
/// Privacy: FCM topics have no access control, so the server only sends a
/// content-free "New messages in the lounge" notice. This service subscribes to
/// the topic only while a user is signed in and unsubscribes on logout.
///
/// Supported on Android, iOS and macOS. Web needs a service worker + VAPID key and
/// does not support topic subscription from the client; Windows has no FCM plugin.
class FCMService {
  static final FCMService _instance = FCMService._internal();
  factory FCMService() => _instance;
  FCMService._internal();

  bool _subscribed = false;
  AuthController? _auth;

  static bool get isSupportedPlatform =>
      !kIsWeb &&
      (defaultTargetPlatform == TargetPlatform.android ||
          defaultTargetPlatform == TargetPlatform.iOS ||
          defaultTargetPlatform == TargetPlatform.macOS);

  /// Call after `Firebase.initializeApp`. Follows [auth] to (un)subscribe.
  Future<void> initialize(AuthController auth) async {
    if (!isSupportedPlatform) return;

    FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);
    FirebaseMessaging.onMessage.listen((RemoteMessage message) {
      // In the foreground the WebSocket already delivers the message.
      debugPrint('Foreground push ignored: ${message.messageId}');
    });

    _auth = auth;
    auth.addListener(_onAuthChanged);
    auth.addLogoutHook(unsubscribe);
    if (!auth.isAuthenticated) {
      // Earlier builds subscribed before login and never unsubscribed; clear
      // any such legacy subscription on a signed-out device.
      await unsubscribe();
    }
    await _onAuthChangedAsync();
  }

  void _onAuthChanged() {
    _onAuthChangedAsync();
  }

  Future<void> _onAuthChangedAsync() async {
    final auth = _auth;
    if (auth == null) return;
    try {
      if (auth.isAuthenticated && !_subscribed) {
        final settings = await FirebaseMessaging.instance.requestPermission(
          alert: true,
          badge: true,
          sound: true,
        );
        final allowed =
            settings.authorizationStatus == AuthorizationStatus.authorized ||
                settings.authorizationStatus == AuthorizationStatus.provisional;
        if (allowed) {
          await FirebaseMessaging.instance.subscribeToTopic(kLoungeTopic);
          _subscribed = true;
          debugPrint('Subscribed to FCM topic: $kLoungeTopic');
        }
      } else if (!auth.isAuthenticated && _subscribed) {
        await unsubscribe();
      }
    } catch (e) {
      debugPrint('FCM subscription update failed: $e');
    }
  }

  /// Stops lounge pushes for this device (called on logout).
  Future<void> unsubscribe() async {
    if (!isSupportedPlatform) return;
    try {
      await FirebaseMessaging.instance.unsubscribeFromTopic(kLoungeTopic);
    } catch (e) {
      debugPrint('FCM unsubscribe failed: $e');
    }
    _subscribed = false;
  }
}
