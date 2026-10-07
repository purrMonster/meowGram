import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/widgets/chat_input_bar.dart';
import 'package:meowgram_client/src/widgets/connection_badge.dart';
import 'package:meowgram_client/src/widgets/message_bubble.dart';

/// The central chat lounge screen integrating [ChatBloc], real-time WebSocket streams,
/// historical message hydration, offline cache display, and responsive keyboard handling.
class ChatScreen extends StatefulWidget {
  final AuthController authController;
  final ChatWebSocketService? socketService;
  final ChatBloc? chatBloc;
  final bool isDesktop;
  final String activeRoom;
  final Widget? drawer;

  const ChatScreen({
    super.key,
    required this.authController,
    this.socketService,
    this.chatBloc,
    this.isDesktop = false,
    this.activeRoom = 'general-lounge',
    this.drawer,
  });

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  late final ChatWebSocketService _socketService;
  late final ChatBloc _chatBloc;
  final ScrollController _scrollController = ScrollController();
  int _lastMessageCount = 0;

  @override
  void initState() {
    super.initState();
    _socketService = widget.socketService ?? ChatWebSocketService();
    _chatBloc = widget.chatBloc ??
        ChatBloc(
          socketService: _socketService,
          tokenProvider: () => widget.authController.accessToken,
          currentSubProvider: () => widget.authController.userProfile?.sub,
        );

    // Initial connection with cache-to-live handoff (idempotent in ChatBloc)
    _chatBloc.add(
      ChatInitializeRequested(accessToken: widget.authController.accessToken),
    );
  }

  @override
  void dispose() {
    // Only dispose if created locally and not passed from ResponsiveLayout
    if (widget.chatBloc == null) {
      _chatBloc.close();
    }
    if (widget.socketService == null) {
      _socketService.dispose();
    }
    _scrollController.dispose();
    super.dispose();
  }

  /// Automatically scrolls the chat timeline to the bottom.
  void _scrollToBottom({bool animated = true}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      if (animated) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 250),
          curve: Curves.easeOutCubic,
        );
      } else {
        _scrollController.jumpTo(_scrollController.position.maxScrollExtent);
      }
    });
  }

  void _sendMessage(String text) {
    _chatBloc.add(ChatSendMessage(text));
    _scrollToBottom();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final profile = widget.authController.userProfile;
    final username = profile?.username ?? 'Whiskers';
    final currentSub = profile?.sub ?? '';

    return BlocProvider.value(
      value: _chatBloc,
      child: Scaffold(
        resizeToAvoidBottomInset: true,
        drawer: widget.drawer,
        appBar: AppBar(
          leading: widget.isDesktop
              ? Padding(
                  padding: const EdgeInsets.all(12.0),
                  child: Icon(
                    Icons.tag_rounded,
                    color: theme.colorScheme.primary,
                  ),
                )
              : (widget.drawer != null
                  ? Builder(
                      builder: (ctx) => IconButton(
                        icon: const Icon(Icons.menu_rounded),
                        tooltip: 'Open Channels',
                        onPressed: () => Scaffold.of(ctx).openDrawer(),
                      ),
                    )
                  : IconButton(
                      icon: const Icon(Icons.arrow_back_rounded),
                      tooltip: 'Back to Login',
                      onPressed: () {
                        _chatBloc.add(const ChatDisconnectRequested());
                        context.go('/login');
                      },
                    )),
          title: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '# ${widget.activeRoom}',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
                overflow: TextOverflow.ellipsis,
              ),
              Text(
                'User: @$username • sub: $currentSub',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontSize: 11,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
          actions: [
            Padding(
              padding: const EdgeInsets.only(right: 4.0),
              child: BlocBuilder<ChatBloc, ChatState>(
                builder: (context, state) {
                  return ConnectionBadge(status: state.status);
                },
              ),
            ),
            IconButton(
              icon: const Icon(Icons.refresh_rounded, size: 20),
              tooltip: 'Reconnect WebSocket',
              onPressed: () {
                _chatBloc.add(
                  ChatConnectRequested(
                    accessToken: widget.authController.accessToken,
                  ),
                );
              },
            ),
            if (!widget.isDesktop && widget.drawer == null)
              IconButton(
                icon: const Icon(Icons.logout_rounded, size: 20),
                tooltip: 'Logout session',
                onPressed: () {
                  _chatBloc.add(const ChatDisconnectRequested());
                  widget.authController.logout();
                },
              ),
          ],
        ),
        body: SafeArea(
          top: false,
          bottom: true,
          child: Column(
            children: [
              // Connection & Offline Cache Status Banner
              BlocBuilder<ChatBloc, ChatState>(
                builder: (context, state) {
                  // Never render the access token: connectedUrl is redacted and
                  // the fallback is the bare endpoint.
                  final target = state.connectedUrl ?? AppConfig.wsBaseUrl;
                  return Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 16, vertical: 6),
                    color: theme.colorScheme.surfaceContainerHighest
                        .withValues(alpha: 0.4),
                    child: Row(
                      children: [
                        Icon(
                          state.isLoadedFromCache
                              ? Icons.offline_bolt_outlined
                              : Icons.lock_outline_rounded,
                          size: 15,
                          color: theme.colorScheme.primary,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            state.isLoadedFromCache && !state.isConnected
                                ? 'Loaded from local cache (offline)'
                                : 'Endpoint: $target',
                            style: const TextStyle(
                              fontSize: 11,
                              fontFamily: 'monospace',
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                  );
                },
              ),

              // Chat Messages Stream & Timeline
              Expanded(
                child: BlocConsumer<ChatBloc, ChatState>(
                  listener: (context, state) {
                    // Auto-scroll on new message arrivals or initial hydration burst
                    if (state.messages.length > _lastMessageCount) {
                      _scrollToBottom();
                      _lastMessageCount = state.messages.length;
                    }
                  },
                  builder: (context, state) {
                    final messages = state.messages;

                    if (messages.isEmpty) {
                      return Center(
                        child: Padding(
                          padding: const EdgeInsets.all(32.0),
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(
                                Icons.chat_bubble_outline_rounded,
                                size: 56,
                                color:
                                    theme.colorScheme.outline.withValues(alpha: 0.5),
                              ),
                              const SizedBox(height: 16),
                              Text(
                                'Lounge Connected',
                                style: theme.textTheme.titleMedium?.copyWith(
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                              const SizedBox(height: 8),
                              Text(
                                'Logged in as @$username ($currentSub).\nNo messages yet. Send the first meow!',
                                textAlign: TextAlign.center,
                                style: theme.textTheme.bodyMedium?.copyWith(
                                  color: theme.colorScheme.onSurfaceVariant,
                                ),
                              ),
                              const SizedBox(height: 20),
                              Wrap(
                                spacing: 8,
                                children: [
                                  ActionChip(
                                    avatar: const Icon(Icons.pets_rounded,
                                        size: 16),
                                    label: const Text('Meow! 🐾'),
                                    onPressed: () => _sendMessage(
                                        'Meow from @$username! 🐾'),
                                  ),
                                  ActionChip(
                                    avatar: const Icon(Icons.favorite_rounded,
                                        size: 16),
                                    label: const Text('Purr... 😺'),
                                    onPressed: () =>
                                        _sendMessage('Purr... 😺'),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                      );
                    }

                    return ListView.builder(
                      controller: _scrollController,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8.0,
                        vertical: 12.0,
                      ),
                      itemCount: messages.length,
                      itemBuilder: (context, index) {
                        final msg = messages[index];
                        return MessageBubble(
                          message: msg,
                          currentSub: currentSub,
                        );
                      },
                    );
                  },
                ),
              ),

              // Quick suggestion action chips
              Padding(
                padding: const EdgeInsets.symmetric(
                    horizontal: 16.0, vertical: 4.0),
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: Row(
                    children: [
                      ActionChip(
                        avatar: const Icon(Icons.pets, size: 14),
                        label: const Text('Purr... 😺',
                            style: TextStyle(fontSize: 12)),
                        onPressed: () => _sendMessage('Purr... 😺'),
                      ),
                      const SizedBox(width: 8),
                      ActionChip(
                        avatar: const Icon(Icons.favorite, size: 14),
                        label: const Text('Cat treats please! 🐟',
                            style: TextStyle(fontSize: 12)),
                        onPressed: () => _sendMessage('Cat treats please! 🐟'),
                      ),
                      const SizedBox(width: 8),
                      ActionChip(
                        avatar: const Icon(Icons.celebration, size: 14),
                        label: const Text('Happy Caturday! 🐱',
                            style: TextStyle(fontSize: 12)),
                        onPressed: () => _sendMessage('Happy Caturday! 🐱'),
                      ),
                    ],
                  ),
                ),
              ),

              // Responsive Chat Input Bar with mobile software keyboard handling
              BlocBuilder<ChatBloc, ChatState>(
                builder: (context, state) {
                  return ChatInputBar(
                    isConnected: state.isConnected,
                    onSendMessage: _sendMessage,
                    onSendPressed: _scrollToBottom,
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}
