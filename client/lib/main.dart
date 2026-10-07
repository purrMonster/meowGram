import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/routes/app_router.dart';
import 'package:meowgram_client/src/push/fcm_service.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:meowgram_client/firebase_options.dart';

import 'package:meowgram_client/src/services/notification_service.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Print startup configuration banner for debug visibility
  debugPrint('====================================================');
  debugPrint('  meowGram Client Bootstrapping');
  debugPrint('  Environment:       ${AppConfig.environment}');
  debugPrint('  Domain:            ${AppConfig.appDomain}');
  debugPrint('  HTTP Port:         ${AppConfig.port}');
  debugPrint('  Secure Schemes:    ${AppConfig.useSecureSchemes}');
  debugPrint('  API Base URL:      ${AppConfig.apiBaseUrl}');
  debugPrint('  WebSocket URL:     ${AppConfig.wsBaseUrl}');
  debugPrint('  Authelia Issuer:   ${AppConfig.autheliaIssuerUrl}');
  debugPrint('  Authelia ClientID: ${AppConfig.autheliaClientId}');
  debugPrint('====================================================');

  try {
    await NotificationService().initialize();
  } catch (e) {
    debugPrint('Notification initialization notice: $e');
  }

  try {
    await Hive.initFlutter();
  } catch (e) {
    debugPrint('Hive initialization notice: $e');
  }

  // Initialize Firebase & FCM
  try {
    await Firebase.initializeApp(
      options: DefaultFirebaseOptions.currentPlatform,
    );
    await FCMService().initialize();
  } catch (e) {
    debugPrint('Firebase initialization failed: $e');
  }

  final authController = AuthController();
  await authController.initialize();

  runApp(MeowGramApp(authController: authController));
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
