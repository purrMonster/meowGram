import 'package:flutter/material.dart';
import 'package:meowgram_client/src/models/chat_message.dart';

/// Renders a single chat message bubble conforming to meowGram styling rules.
///
/// Features:
/// - Current user's messages align to the right with a distinct primary color.
/// - Other users' messages align to the left with secondary/surface-container background and sender label.
/// - System notices (joins, leaves) render as centered, subtle cat-themed pills.
/// - Error frames render with warning accents.
class MessageBubble extends StatelessWidget {
  final ChatMessage message;
  final String? currentSub;

  const MessageBubble({
    super.key,
    required this.message,
    this.currentSub,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    // 1. System Notices (User joined, user left)
    if (message.isSystem) {
      return Center(
        child: Container(
          margin: const EdgeInsets.symmetric(vertical: 6.0, horizontal: 16.0),
          padding: const EdgeInsets.symmetric(horizontal: 14.0, vertical: 6.0),
          decoration: BoxDecoration(
            color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.5),
            borderRadius: BorderRadius.circular(16.0),
            border: Border.all(
              color: theme.colorScheme.outlineVariant.withValues(alpha: 0.3),
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.pets_rounded,
                size: 14,
                color: theme.colorScheme.primary.withValues(alpha: 0.8),
              ),
              const SizedBox(width: 6),
              Flexible(
                child: Text(
                  message.textContent,
                  style: theme.textTheme.bodySmall?.copyWith(
                    fontStyle: FontStyle.italic,
                    color: theme.colorScheme.onSurfaceVariant,
                    fontSize: 12,
                  ),
                  textAlign: TextAlign.center,
                ),
              ),
            ],
          ),
        ),
      );
    }

    // 2. Server Error Notifications
    if (message.isError) {
      return Center(
        child: Container(
          margin: const EdgeInsets.symmetric(vertical: 6.0, horizontal: 16.0),
          padding: const EdgeInsets.symmetric(horizontal: 14.0, vertical: 8.0),
          decoration: BoxDecoration(
            color: theme.colorScheme.errorContainer.withValues(alpha: 0.8),
            borderRadius: BorderRadius.circular(12.0),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.warning_amber_rounded,
                  size: 16, color: theme.colorScheme.onErrorContainer),
              const SizedBox(width: 6),
              Flexible(
                child: Text(
                  message.textContent,
                  style: TextStyle(
                    fontSize: 12,
                    color: theme.colorScheme.onErrorContainer,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }

    // 3. Chat / History Messages
    final isSelf = message.isFromSelf(currentSub);
    final maxWidth = MediaQuery.of(context).size.width * 0.78;

    final bubbleBg = isSelf
        ? theme.colorScheme.primary
        : theme.colorScheme.surfaceContainerHighest;

    final textColor = isSelf
        ? theme.colorScheme.onPrimary
        : theme.colorScheme.onSurface;

    final subTextColor = isSelf
        ? theme.colorScheme.onPrimary.withValues(alpha: 0.75)
        : theme.colorScheme.onSurfaceVariant;

    final displayName = isSelf
        ? 'You'
        : (message.username != null && message.username!.isNotEmpty
            ? '@${message.username}'
            : (message.senderId != null ? '@${message.senderId}' : 'Guest'));

    return Align(
      alignment: isSelf ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 4.0, horizontal: 12.0),
        constraints: BoxConstraints(maxWidth: maxWidth),
        child: Column(
          crossAxisAlignment:
              isSelf ? CrossAxisAlignment.end : CrossAxisAlignment.start,
          children: [
            // Sender name (only for peer messages to reduce visual noise for self)
            if (!isSelf)
              Padding(
                padding: const EdgeInsets.only(left: 6.0, bottom: 2.0),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.pets_rounded,
                      size: 11,
                      color: theme.colorScheme.primary,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      displayName,
                      style: theme.textTheme.labelSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: theme.colorScheme.primary,
                        fontSize: 11,
                      ),
                    ),
                  ],
                ),
              ),

            // Chat Bubble Container
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 14.0, vertical: 10.0),
              decoration: BoxDecoration(
                color: bubbleBg,
                borderRadius: BorderRadius.only(
                  topLeft: const Radius.circular(18),
                  topRight: const Radius.circular(18),
                  bottomLeft: Radius.circular(isSelf ? 18 : 4),
                  bottomRight: Radius.circular(isSelf ? 4 : 18),
                ),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withValues(alpha: 0.04),
                    blurRadius: 4,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: Column(
                crossAxisAlignment:
                    isSelf ? CrossAxisAlignment.end : CrossAxisAlignment.start,
                children: [
                  SelectableText(
                    message.textContent,
                    style: TextStyle(
                      fontSize: 14.5,
                      color: textColor,
                      height: 1.35,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        message.formattedTime,
                        style: TextStyle(
                          fontSize: 10,
                          color: subTextColor,
                        ),
                      ),
                      if (isSelf) ...[
                        const SizedBox(width: 4),
                        Icon(
                          Icons.done_all_rounded,
                          size: 12,
                          color: subTextColor,
                        ),
                      ],
                    ],
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
