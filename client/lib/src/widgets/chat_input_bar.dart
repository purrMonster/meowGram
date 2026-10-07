import 'package:flutter/material.dart';

/// Bottom message composition bar with software keyboard awareness.
///
/// Features:
/// - Handles mobile software keyboard cleanly with [SafeArea] and [MediaQuery.viewInsetsOf].
/// - Responsive layout adapting smoothly to Web, Desktop, and Mobile form factors.
/// - Text field with instant clear and send dispatch.
class ChatInputBar extends StatefulWidget {
  final ValueChanged<String> onSendMessage;
  final bool isConnected;
  final VoidCallback? onSendPressed;

  const ChatInputBar({
    super.key,
    required this.onSendMessage,
    this.isConnected = true,
    this.onSendPressed,
  });

  @override
  State<ChatInputBar> createState() => _ChatInputBarState();
}

class _ChatInputBarState extends State<ChatInputBar> {
  final TextEditingController _controller = TextEditingController();
  final FocusNode _focusNode = FocusNode();
  bool _canSend = false;

  @override
  void initState() {
    super.initState();
    _controller.addListener(_onTextChanged);
  }

  void _onTextChanged() {
    final hasText = _controller.text.trim().isNotEmpty;
    if (hasText != _canSend) {
      setState(() {
        _canSend = hasText;
      });
    }
  }

  void _submit() {
    final text = _controller.text.trim();
    if (text.isEmpty || !widget.isConnected) return;

    widget.onSendMessage(text);
    _controller.clear();
    widget.onSendPressed?.call();

    // Maintain focus for continuous desktop/web typing
    _focusNode.requestFocus();
  }

  @override
  void dispose() {
    _controller.removeListener(_onTextChanged);
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isKeyboardOpen = MediaQuery.viewInsetsOf(context).bottom > 0;

    return Container(
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        border: Border(
          top: BorderSide(
            color: theme.colorScheme.outlineVariant.withValues(alpha: 0.4),
          ),
        ),
      ),
      child: SafeArea(
        top: false,
        bottom: !isKeyboardOpen,
        child: Padding(
          padding: EdgeInsets.only(
            left: 12.0,
            right: 12.0,
            top: 8.0,
            bottom: isKeyboardOpen ? 4.0 : 8.0,
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                child: Container(
                  decoration: BoxDecoration(
                    color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.5),
                    borderRadius: BorderRadius.circular(24.0),
                    border: Border.all(
                      color: _focusNode.hasFocus
                          ? theme.colorScheme.primary.withValues(alpha: 0.5)
                          : Colors.transparent,
                    ),
                  ),
                  child: Row(
                    children: [
                      const SizedBox(width: 14),
                      Icon(
                        Icons.pets_rounded,
                        size: 18,
                        color: theme.colorScheme.primary.withValues(alpha: 0.7),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: TextField(
                          controller: _controller,
                          focusNode: _focusNode,
                          enabled: widget.isConnected,
                          minLines: 1,
                          maxLines: 4,
                          textInputAction: TextInputAction.send,
                          onSubmitted: (_) => _submit(),
                          decoration: InputDecoration(
                            hintText: widget.isConnected
                                ? 'Send a meow to the lounge...'
                                : 'Connecting to WebSocket...',
                            hintStyle: TextStyle(
                              fontSize: 14,
                              color: theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.7),
                            ),
                            border: InputBorder.none,
                            isDense: true,
                            contentPadding: const EdgeInsets.symmetric(
                              vertical: 12.0,
                              horizontal: 0.0,
                            ),
                          ),
                        ),
                      ),
                      if (_canSend)
                        IconButton(
                          icon: const Icon(Icons.close_rounded, size: 18),
                          tooltip: 'Clear input',
                          color: theme.colorScheme.onSurfaceVariant,
                          onPressed: () => _controller.clear(),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Padding(
                padding: const EdgeInsets.only(bottom: 2.0),
                child: IconButton.filled(
                  icon: const Icon(Icons.send_rounded, size: 20),
                  tooltip: widget.isConnected ? 'Send message' : 'Reconnecting...',
                  onPressed: (_canSend && widget.isConnected) ? _submit : null,
                  style: IconButton.styleFrom(
                    backgroundColor: theme.colorScheme.primary,
                    foregroundColor: theme.colorScheme.onPrimary,
                    disabledBackgroundColor:
                        theme.colorScheme.surfaceContainerHighest,
                    disabledForegroundColor:
                        theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.4),
                    padding: const EdgeInsets.all(12),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
