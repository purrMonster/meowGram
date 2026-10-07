import 'package:flutter/material.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';

class ConnectionBadge extends StatelessWidget {
  final SocketStatus status;

  const ConnectionBadge({super.key, required this.status});

  @override
  Widget build(BuildContext context) {
    final (color, label, icon) = switch (status) {
      SocketStatus.connected => (
          Colors.greenAccent.shade700,
          'Connected',
          Icons.check_circle_rounded,
        ),
      SocketStatus.connecting => (
          Colors.amber.shade700,
          'Connecting...',
          Icons.sync_rounded,
        ),
      SocketStatus.error => (
          Colors.redAccent.shade700,
          'Connection Error',
          Icons.error_outline_rounded,
        ),
      SocketStatus.disconnected => (
          Colors.grey.shade600,
          'Disconnected',
          Icons.pause_circle_outline_rounded,
        ),
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: color.withValues(alpha: 0.35), width: 1),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              color: color,
              fontSize: 12,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.2,
            ),
          ),
        ],
      ),
    );
  }
}
