import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/widgets/connection_badge.dart';

class ChatScreen extends StatefulWidget {
  final AuthController authController;

  const ChatScreen({super.key, required this.authController});

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final ChatWebSocketService _socketService = ChatWebSocketService();
  final TextEditingController _textController = TextEditingController();
  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    // Auto-connect to backend WebSocket endpoint with Bearer token injected
    _socketService.connect(accessToken: widget.authController.accessToken);
  }

  @override
  void dispose() {
    _socketService.dispose();
    _textController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _sendMessage([String? presetText]) {
    final text = presetText ?? _textController.text.trim();
    if (text.isEmpty) return;

    _socketService.sendMessage(text);
    if (presetText == null) {
      _textController.clear();
    }

    _scrollToBottom();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 250),
          curve: Curves.easeOut,
        );
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final profile = widget.authController.userProfile;
    final username = profile?.username ?? 'Whiskers';
    final sub = profile?.sub ?? 'unknown';

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_rounded),
          tooltip: 'Back to Login',
          onPressed: () {
            _socketService.disconnect();
            context.go('/login');
          },
        ),
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.pets_rounded, size: 18),
                const SizedBox(width: 8),
                Text(
                  'meowGram Lounge',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            Text(
              'User: @$username • sub: $sub',
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
            padding: const EdgeInsets.only(right: 6.0),
            child: AnimatedBuilder(
              animation: _socketService,
              builder: (context, _) =>
                  ConnectionBadge(status: _socketService.status),
            ),
          ),
          IconButton(
            icon: const Icon(Icons.refresh_rounded),
            tooltip: 'Reconnect WebSocket',
            onPressed: () => _socketService.connect(
                accessToken: widget.authController.accessToken),
          ),
          IconButton(
            icon: const Icon(Icons.logout_rounded),
            tooltip: 'Logout session',
            onPressed: () {
              _socketService.disconnect();
              widget.authController.logout();
            },
          ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: [
            // Target & Auth Token Banner
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              color: theme.colorScheme.surfaceContainerHighest.withOpacity(0.4),
              child: Row(
                children: [
                  Icon(Icons.lock_outline_rounded,
                      size: 16, color: theme.colorScheme.primary),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'WS Target: ${_socketService.connectedUrl ?? AppConfig.authenticatedWsUrl(widget.authController.accessToken ?? '')}',
                      style: const TextStyle(
                          fontSize: 11, fontFamily: 'monospace'),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
            ),

            // Message History Area
            Expanded(
              child: AnimatedBuilder(
                animation: _socketService,
                builder: (context, _) {
                  final messages = _socketService.messages;

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
                              color: theme.colorScheme.outline.withOpacity(0.5),
                            ),
                            const SizedBox(height: 16),
                            Text(
                              'Authenticated via Authelia OIDC',
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                            const SizedBox(height: 8),
                            Text(
                              'Logged in as @$username ($sub).\nConnected to Go backend with token verification & auto-provisioning.',
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
                                  avatar:
                                      const Icon(Icons.pets_rounded, size: 16),
                                  label: const Text('Meow! 🐾'),
                                  onPressed: () =>
                                      _sendMessage('Meow from @$username! 🐾'),
                                ),
                                ActionChip(
                                  avatar:
                                      const Icon(Icons.bolt_rounded, size: 16),
                                  label: const Text('Ping Server'),
                                  onPressed: () => _sendMessage(
                                      'ping-${DateTime.now().millisecondsSinceEpoch}'),
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
                        horizontal: 16, vertical: 12),
                    itemCount: messages.length,
                    itemBuilder: (context, index) {
                      final msg = messages[index];
                      return _MessageBubble(message: msg);
                    },
                  );
                },
              ),
            ),

            // Quick suggestion chips
            Padding(
              padding:
                  const EdgeInsets.symmetric(horizontal: 16.0, vertical: 4.0),
              child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    ActionChip(
                      avatar: const Icon(Icons.pets, size: 14),
                      label:
                          const Text('Purr...', style: TextStyle(fontSize: 12)),
                      onPressed: () => _sendMessage('Purr... 😺'),
                    ),
                    const SizedBox(width: 8),
                    ActionChip(
                      avatar: const Icon(Icons.favorite, size: 14),
                      label: const Text('Cat treats please!',
                          style: TextStyle(fontSize: 12)),
                      onPressed: () => _sendMessage('Cat treats please! 🐟'),
                    ),
                    const SizedBox(width: 8),
                    ActionChip(
                      avatar: const Icon(Icons.code, size: 14),
                      label: const Text('echo test',
                          style: TextStyle(fontSize: 12)),
                      onPressed: () => _sendMessage(
                          'WebSocket echo test: ${DateTime.now().toIso8601String()}'),
                    ),
                  ],
                ),
              ),
            ),

            // Message Input Bar
            Container(
              padding: const EdgeInsets.all(12.0),
              decoration: BoxDecoration(
                color: theme.colorScheme.surface,
                border: Border(
                  top: BorderSide(
                    color: theme.colorScheme.outlineVariant.withOpacity(0.5),
                  ),
                ),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _textController,
                      decoration: InputDecoration(
                        hintText: 'Type a message to echo...',
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(24),
                          borderSide: BorderSide.none,
                        ),
                        filled: true,
                        fillColor: theme.colorScheme.surfaceContainerHighest
                            .withOpacity(0.5),
                        contentPadding: const EdgeInsets.symmetric(
                            horizontal: 20, vertical: 12),
                      ),
                      onSubmitted: (_) => _sendMessage(),
                    ),
                  ),
                  const SizedBox(width: 8),
                  AnimatedBuilder(
                    animation: _socketService,
                    builder: (context, _) {
                      final isConnected =
                          _socketService.status == SocketStatus.connected;
                      return IconButton.filled(
                        icon: const Icon(Icons.send_rounded),
                        onPressed: isConnected ? () => _sendMessage() : null,
                        tooltip: isConnected
                            ? 'Send message'
                            : 'WebSocket not connected',
                      );
                    },
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  final ChatMessage message;

  const _MessageBubble({required this.message});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isSelf = message.isFromSelf;

    return Align(
      alignment: isSelf ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 4),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        constraints: BoxConstraints(
          maxWidth: MediaQuery.of(context).size.width * 0.75,
        ),
        decoration: BoxDecoration(
          color: isSelf
              ? theme.colorScheme.primaryContainer
              : theme.colorScheme.secondaryContainer,
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(16),
            topRight: const Radius.circular(16),
            bottomLeft: Radius.circular(isSelf ? 16 : 4),
            bottomRight: Radius.circular(isSelf ? 4 : 16),
          ),
        ),
        child: Column(
          crossAxisAlignment:
              isSelf ? CrossAxisAlignment.end : CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  isSelf ? Icons.arrow_upward_rounded : Icons.reply_rounded,
                  size: 12,
                  color: isSelf
                      ? theme.colorScheme.onPrimaryContainer.withOpacity(0.7)
                      : theme.colorScheme.onSecondaryContainer.withOpacity(0.7),
                ),
                const SizedBox(width: 4),
                Text(
                  isSelf ? 'Sent' : 'Server Response',
                  style: TextStyle(
                    fontSize: 10,
                    fontWeight: FontWeight.w600,
                    color: isSelf
                        ? theme.colorScheme.onPrimaryContainer.withOpacity(0.7)
                        : theme.colorScheme.onSecondaryContainer
                            .withOpacity(0.7),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              message.text,
              style: TextStyle(
                fontSize: 14,
                color: isSelf
                    ? theme.colorScheme.onPrimaryContainer
                    : theme.colorScheme.onSecondaryContainer,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
