import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/screens/login_screen.dart';
import 'package:meowgram_client/src/screens/responsive_layout.dart';

/// Creates the central application router using [GoRouter] with declarative route guards.
///
/// Mechanics:
/// By passing [authController] to [refreshListenable], [GoRouter] automatically
/// re-evaluates the [redirect] callback whenever the authentication state changes.
/// - Unauthenticated users attempting to access `/chat` are redirected to `/login`.
/// - Authenticated users landing on `/login` are automatically redirected to `/chat`.
GoRouter createAppRouter(AuthController authController) {
  return GoRouter(
    initialLocation: '/login',
    refreshListenable: authController,
    redirect: (BuildContext context, GoRouterState state) {
      final isAuthenticated = authController.isAuthenticated;
      final isLoggingIn = state.uri.path == '/login';

      // 1. Guard: If not authenticated and trying to access protected route -> /login
      if (!isAuthenticated && !isLoggingIn) {
        return '/login';
      }

      // 2. Auto-route: If authenticated and currently on login or root -> /chat
      if (isAuthenticated && (isLoggingIn || state.uri.path == '/')) {
        return '/chat';
      }

      return null;
    },
    routes: <RouteBase>[
      GoRoute(
        path: '/login',
        name: 'login',
        builder: (BuildContext context, GoRouterState state) {
          return LoginScreen(authController: authController);
        },
      ),
      GoRoute(
        path: '/chat',
        name: 'chat',
        builder: (BuildContext context, GoRouterState state) {
          return ResponsiveLayout(authController: authController);
        },
      ),
    ],
    errorBuilder: (context, state) => Scaffold(
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline_rounded,
                size: 48, color: Colors.redAccent),
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
}
