import 'package:flutter/material.dart';

/// The meowGram app logo: a cat-head chat bubble on a violet rounded square.
///
/// Rendered from `brand/meowgram-icon.svg` by `brand/render.sh`; the PNG
/// carries its own rounded corners, so it sits on any background.
class MeowGramLogo extends StatelessWidget {
  static const assetPath = 'assets/brand/logo.png';

  final double size;

  /// Adds a soft brand-coloured glow under the logo (for hero placements).
  final bool elevated;

  const MeowGramLogo({super.key, this.size = 40, this.elevated = false});

  @override
  Widget build(BuildContext context) {
    final image = Image.asset(
      assetPath,
      width: size,
      height: size,
      filterQuality: FilterQuality.medium,
      semanticLabel: 'meowGram',
    );
    if (!elevated) return image;

    return DecoratedBox(
      decoration: BoxDecoration(
        // Matches the corner radius baked into the artwork (230 / 1024).
        borderRadius: BorderRadius.circular(size * 0.225),
        boxShadow: [
          BoxShadow(
            color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.35),
            blurRadius: size * 0.22,
            offset: Offset(0, size * 0.09),
          ),
        ],
      ),
      child: image,
    );
  }
}
