import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/screens/chat_screen.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';
import 'package:meowgram_client/src/widgets/sidebar.dart';

/// Responsive wrapper orchestrating the dual-pane desktop layout and single-pane mobile layout.
///
/// Responsive Breakpoint Architecture:
/// - Threshold: 800.0 logical pixels.
/// - Desktop / Tablet (>= 800px): Renders a persistent dual-pane layout with the [Sidebar]
///   on the left (270px width) containing channels and active members, and the main [ChatScreen]
///   expanded on the right.
/// - Mobile (< 800px): Renders a streamlined single-pane layout preserving push/pop navigation,
///   with the [Sidebar] accessible as a modal drawer.
/// - State Persistence: A single [ChatBloc] instance is scoped to [ResponsiveLayout],
///   ensuring the timeline, WebSocket connection, and offline cache remain intact when
///   the user resizes their browser window or rotates their tablet device.
class ResponsiveLayout extends StatefulWidget {
  final AuthController authController;
  final ChatWebSocketService? socketService;
  final ChatBloc? chatBloc;
  final LocalMessageRepository? localRepo;

  static const double desktopBreakpoint = 800.0;

  const ResponsiveLayout({
    super.key,
    required this.authController,
    this.socketService,
    this.chatBloc,
    this.localRepo,
  });

  @override
  State<ResponsiveLayout> createState() => _ResponsiveLayoutState();
}

class _ResponsiveLayoutState extends State<ResponsiveLayout>
    with WidgetsBindingObserver {
  late final ChatWebSocketService _socketService;
  late final ChatBloc _chatBloc;
  String _selectedRoom = 'general-lounge';

  @override
  void initState() {
    super.initState();
    _socketService = widget.socketService ?? ChatWebSocketService();
    _chatBloc = widget.chatBloc ??
        ChatBloc(
          socketService: _socketService,
          localRepo: widget.localRepo,
          tokenProvider: () => widget.authController.accessToken,
          currentSubProvider: () => widget.authController.userProfile?.sub,
        );

    // Initial launch: loads local cache instantly, then connects live WebSocket in background
    _chatBloc.add(
      ChatInitializeRequested(accessToken: widget.authController.accessToken),
    );

    WidgetsBinding.instance.addObserver(this);
    widget.authController.addListener(_onAuthChanged);
    _lastToken = widget.authController.accessToken;
  }

  String? _lastToken;

  /// After a token refresh, reconnect a dropped socket with the new token.
  void _onAuthChanged() {
    final token = widget.authController.accessToken;
    if (token == null || token == _lastToken) return;
    _lastToken = token;
    if (!_chatBloc.isClosed && !_chatBloc.state.isConnected) {
      _chatBloc.add(ChatConnectRequested(accessToken: token));
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed &&
        !_chatBloc.isClosed &&
        !_chatBloc.state.isConnected) {
      _chatBloc.add(
        ChatConnectRequested(accessToken: widget.authController.accessToken),
      );
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    widget.authController.removeListener(_onAuthChanged);
    if (widget.chatBloc == null) {
      _chatBloc.close();
    }
    if (widget.socketService == null) {
      _socketService.dispose();
    }
    super.dispose();
  }

  void _onRoomSelected(String roomId) {
    setState(() {
      _selectedRoom = roomId;
    });
  }

  @override
  Widget build(BuildContext context) {
    return BlocProvider.value(
      value: _chatBloc,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final isDesktop = constraints.maxWidth >= ResponsiveLayout.desktopBreakpoint;

          if (isDesktop) {
            // =================================================================
            // Dual-Pane Layout (Screens >= 800px)
            // =================================================================
            return Scaffold(
              body: Row(
                children: [
                  Sidebar(
                    authController: widget.authController,
                    selectedRoom: _selectedRoom,
                    onRoomSelected: _onRoomSelected,
                    onLogoutPressed: () {
                      _chatBloc.add(const ChatDisconnectRequested());
                      widget.authController.logout();
                    },
                  ),
                  Expanded(
                    child: ChatScreen(
                      authController: widget.authController,
                      socketService: _socketService,
                      chatBloc: _chatBloc,
                      isDesktop: true,
                      activeRoom: _selectedRoom,
                    ),
                  ),
                ],
              ),
            );
          } else {
            // =================================================================
            // Mobile Stacked Layout (Screens < 800px)
            // =================================================================
            return ChatScreen(
              authController: widget.authController,
              socketService: _socketService,
              chatBloc: _chatBloc,
              isDesktop: false,
              activeRoom: _selectedRoom,
              drawer: Drawer(
                child: SafeArea(
                  child: Sidebar(
                    authController: widget.authController,
                    selectedRoom: _selectedRoom,
                    onRoomSelected: (room) {
                      _onRoomSelected(room);
                      Navigator.of(context).pop(); // Close drawer on selection
                    },
                    onLogoutPressed: () {
                      Navigator.of(context).pop();
                      _chatBloc.add(const ChatDisconnectRequested());
                      widget.authController.logout();
                    },
                  ),
                ),
              ),
            );
          }
        },
      ),
    );
  }
}
