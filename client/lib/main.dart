import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:meowgram_client/firebase_options.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/push/fcm_service.dart';
import 'package:meowgram_client/src/routes/app_router.dart';
import 'package:meowgram_client/src/services/background_message_service.dart';
import 'package:meowgram_client/src/services/notification_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

/// Startup order (offline-first, AGENTS.md):
/// 1. Local-only work before the first frame: Hive and the stored session.
///    Nothing here waits on the network.
/// 2. `runApp`: the cached timeline renders immediately.
/// 3. Deferred: notification permission, Firebase/FCM and background sync.
///    These may prompt the user or hit the network, so they never block the UI.
Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Print startup configuration banner for debug visibility
  debugPrint('====================================================');
  debugPrint('  meowGram Client Bootstrapping');
  debugPrint('  Environment:       ${AppConfig.environment}');
  debugPrint('  Domain:            ${AppConfig.appDomain}');
  debugPrint('  API Base URL:      ${AppConfig.apiBaseUrl}');
  debugPrint('  WebSocket URL:     ${AppConfig.wsBaseUrl}');
  debugPrint('  Authelia Issuer:   ${AppConfig.autheliaIssuerUrl}');
  debugPrint('  Authelia ClientID: ${AppConfig.autheliaClientId}');
  debugPrint('====================================================');

  try {
    await Hive.initFlutter();
  } catch (e) {
    debugPrint('Hive initialization notice: $e');
  }

  final authController = AuthController();
  // Logout clears this device's copy of the conversation and any shown alerts.
  authController.addLogoutHook(() => HiveLocalMessageRepository().clear());
  authController.addLogoutHook(() => NotificationService().cancelAll());
  await authController.initialize();

  runApp(MeowGramApp(authController: authController));

  unawaited(_initDeferredServices(authController));
}

Future<void> _initDeferredServices(AuthController authController) async {
  try {
    await NotificationService().initialize();
  } catch (e) {
    debugPrint('Notification initialization notice: $e');
  }

  if (FCMService.isSupportedPlatform) {
    try {
      await Firebase.initializeApp(
        options: DefaultFirebaseOptions.currentPlatform,
      );
      await FCMService().initialize(authController);
    } catch (e) {
      debugPrint('Firebase initialization failed: $e');
    }
  }

  try {
    await BackgroundMessageService.configure();
  } catch (e) {
    debugPrint('Background sync configuration notice: $e');
  }
}

class MeowGramApp extends StatefulWidget {
  final AuthController authController;

  const MeowGramApp({super.key, required this.authController});

  @override
  State<MeowGramApp> createState() => _MeowGramAppState();
}

class _MeowGramAppState extends State<MeowGramApp> with WidgetsBindingObserver {
  late final GoRouter _router;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _router = createAppRouter(widget.authController);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    NotificationService().appInForeground = state == AppLifecycleState.resumed;
    if (state == AppLifecycleState.resumed) {
      widget.authController.checkSession();
    }
  }

  @override
  Widget build(BuildContext context) {
    const seedColor = Color(0xFF6750A4); // Rich amethyst / violet

    return MaterialApp.router(
      title: AppConfig.appName,
      debugShowCheckedModeBanner: false,
      routerConfig: _router,
      themeMode: ThemeMode.system,
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: ColorScheme.fromSeed(
          seedColor: seedColor,
          brightness: Brightness.light,
        ),
      ),
      darkTheme: ThemeData(
        useMaterial3: true,
        colorScheme: ColorScheme.fromSeed(
          seedColor: seedColor,
          brightness: Brightness.dark,
        ),
      ),
    );
  }
}
