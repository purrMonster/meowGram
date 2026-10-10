import 'dart:async';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/push/device_registration_client.dart';

@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  // The OS displays content-free notification messages.
}

/// Authenticated per-device registrations; never subscribes to public topics.
class FCMService {
  static final FCMService _instance = FCMService._internal();
  factory FCMService() => _instance;
  FCMService._internal();

  final _registry = DeviceRegistrationClient();
  AuthController? _auth;
  String? _registeredToken;
  String? _registeredBearer;
  Future<void> _work = Future.value();
  Timer? _retry;

  static bool get isSupportedPlatform =>
      !kIsWeb &&
      (defaultTargetPlatform == TargetPlatform.android ||
          defaultTargetPlatform == TargetPlatform.iOS ||
          defaultTargetPlatform == TargetPlatform.macOS);

  Future<void> initialize(AuthController auth) async {
    if (!isSupportedPlatform || _auth != null) return;
    _auth = auth;
    FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);
    // Old app versions subscribed to this topic. Remove their subscriptions.
    try {
      await FirebaseMessaging.instance.unsubscribeFromTopic('room_lounge');
    } catch (_) {}
    FirebaseMessaging.instance.onTokenRefresh.listen(
      (_) => _enqueueSync(force: true),
    );
    auth.addListener(_onAuthChanged);
    auth.addLogoutHook(unsubscribe);
    Timer.periodic(const Duration(hours: 12), (_) => _enqueueSync(force: true));
    await _enqueueSync(force: true);
  }

  void _onAuthChanged() {
    unawaited(_enqueueSync());
  }

  Future<void> _enqueueSync({bool force = false}) {
    _work = _work.then((_) => _sync(force: force)).catchError((Object _) {
      // Provider errors can contain device tokens. Never print them.
      _retry?.cancel();
      _retry = Timer(
        const Duration(minutes: 1),
        () => _enqueueSync(force: true),
      );
    });
    return _work;
  }

  Future<void> _sync({required bool force}) async {
    final auth = _auth!;
    if (!auth.isAuthenticated) {
      await _remove();
      return;
    }
    final bearer = auth.accessToken;
    if (bearer == null || bearer.isEmpty) return;
    final settings = await FirebaseMessaging.instance.requestPermission(
      alert: true,
      badge: true,
      sound: true,
    );
    if (settings.authorizationStatus != AuthorizationStatus.authorized &&
        settings.authorizationStatus != AuthorizationStatus.provisional) {
      await _remove();
      return;
    }
    final token = await FirebaseMessaging.instance.getToken();
    if (token == null || !auth.isAuthenticated || auth.accessToken != bearer) {
      return;
    }
    if (!force && token == _registeredToken && bearer == _registeredBearer) {
      return;
    }
    if (_registeredToken != null && token != _registeredToken) {
      try {
        await _registry.unregister(_registeredToken!, _registeredBearer!);
      } catch (_) {}
    }
    await _registry.register(token, bearer);
    _registeredToken = token;
    _registeredBearer = bearer;
    // Logout may have happened while the HTTP request was in flight.
    if (!auth.isAuthenticated) await _remove();
  }

  /// Serialized with registration so a late PUT cannot undo logout's DELETE.
  Future<void> unsubscribe() {
    _retry?.cancel();
    _work = _work.then((_) => _remove()).catchError((Object _) {});
    return _work;
  }

  Future<void> _remove() async {
    final token = _registeredToken;
    final bearer = _registeredBearer;
    _registeredToken = null;
    _registeredBearer = null;
    try {
      if (token != null && bearer != null) {
        await _registry.unregister(token, bearer);
      }
    } finally {
      // Invalidates delivery even if the API is offline or the bearer expired.
      await FirebaseMessaging.instance.deleteToken();
    }
  }
}
