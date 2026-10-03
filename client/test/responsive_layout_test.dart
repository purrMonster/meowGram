import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/screens/chat_screen.dart';
import 'package:meowgram_client/src/screens/responsive_layout.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';
import 'package:meowgram_client/src/widgets/sidebar.dart';
import 'chat_ui_test.dart';

void main() {
  group('ResponsiveLayout Breakpoint Tests', () {
    late AuthController authController;
    late FakeWebSocketService fakeService;
    late MemoryLocalMessageRepository fakeLocalRepo;
    late ChatBloc chatBloc;

    setUp(() {
      authController = AuthController();
      fakeService = FakeWebSocketService();
      fakeLocalRepo = MemoryLocalMessageRepository();
      chatBloc = ChatBloc(
        socketService: fakeService,
        localRepo: fakeLocalRepo,
      );
    });

    tearDown(() {
      chatBloc.close();
      fakeService.dispose();
    });

    testWidgets('Desktop view (>= 800px): Renders persistent dual-pane Sidebar & ChatScreen',
        (WidgetTester tester) async {
      // Set physical size to desktop dimensions: 1200 x 800
      tester.view.physicalSize = const Size(1200, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        MaterialApp(
          home: ResponsiveLayout(
            authController: authController,
            socketService: fakeService,
            chatBloc: chatBloc,
            localRepo: fakeLocalRepo,
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Verify persistent desktop sidebar exists
      expect(find.byType(Sidebar), findsOneWidget);
      expect(find.text('CHANNELS'), findsOneWidget);
      expect(find.text('# general-lounge'), findsWidgets);
      expect(find.text('# cat-memes'), findsOneWidget);
      expect(find.text('LOUNGE MEMBERS', skipOffstage: false), findsOneWidget);

      // Verify main chat screen is rendered alongside
      expect(find.byType(ChatScreen), findsOneWidget);

      // In desktop dual-pane, hamburger menu drawer button should NOT be shown
      expect(find.byIcon(Icons.menu_rounded), findsNothing);
    });

    testWidgets('Mobile view (< 800px): Renders single-pane ChatScreen with accessible drawer',
        (WidgetTester tester) async {
      // Set physical size to mobile dimensions: 400 x 800
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        MaterialApp(
          home: ResponsiveLayout(
            authController: authController,
            socketService: fakeService,
            chatBloc: chatBloc,
            localRepo: fakeLocalRepo,
          ),
        ),
      );

      await tester.pumpAndSettle();

      // In mobile view, persistent sidebar is NOT in main viewport
      // Instead, ChatScreen is rendered with a menu button to open drawer
      expect(find.byType(ChatScreen), findsOneWidget);
      final menuButton = find.byIcon(Icons.menu_rounded);
      expect(menuButton, findsOneWidget);

      // Tap drawer button to open sidebar in drawer
      await tester.tap(menuButton);
      await tester.pumpAndSettle();

      // Drawer is now open, Sidebar is visible
      expect(find.byType(Sidebar), findsOneWidget);
      expect(find.text('# general-lounge'), findsWidgets);
    });
  });
}
