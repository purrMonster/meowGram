import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:meowgram_client/src/screens/chat_screen.dart';
import 'package:meowgram_client/src/screens/login_screen.dart';

/// Central application router using go_router.
/// Provides declarative routing between /login and /chat screens.
final GoRouter appRouter = GoRouter(
  initialLocation: '/login',
  routes: <RouteBase>[
    GoRoute(
      path: '/login',
      name: 'login',
      builder: (BuildContext context, GoRouterState state) {
        return const LoginScreen();
      },
    ),
    GoRoute(
      path: '/chat',
      name: 'chat',
      builder: (BuildContext context, GoRouterState state) {
        final username = state.extra as String? ?? 'Whiskers';
        return ChatScreen(username: username);
      },
    ),
  ],
  errorBuilder: (context, state) => Scaffold(
    body: Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(
            Icons.error_outline_rounded,
            size: 48,
            color: Colors.redAccent,
          ),
          const SizedBox(height: 16),
          Text('Page not found: ${state.uri.path}'),
          const SizedBox(height: 16),
          FilledButton(
            onPressed: () => context.go('/login'),
            child: const Text('Return to Login'),
          ),
        ],
      ),
    ),
  ),
);
