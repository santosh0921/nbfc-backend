import 'package:flutter/material.dart';

/// A static, extremely subtle diagonal light reflection painted across the
/// top of a card — the "premium glass shine" detail called for by the
/// design system. Deliberately a plain [Paint]/gradient decoration rather
/// than a `BackdropFilter` blur or a looping animation: a blur per card
/// would tank scroll performance the moment more than a couple are on
/// screen at once, and a continuous shimmer competes for attention with
/// real content. This paints once and sits still.
///
/// Wrap any card's `Stack` with this as the top layer, positioned to fill
/// the card, and it overlays a soft diagonal gold/white streak in the
/// upper third without affecting hit-testing (`IgnorePointer`).
class PremiumShine extends StatelessWidget {
  const PremiumShine({super.key, this.opacity = 0.35, this.goldTint = false});

  /// Overall strength of the reflection. Keep this low (0.2-0.4) — the
  /// brief is "the user should notice the premium finish without
  /// immediately thinking glassmorphism".
  final double opacity;

  /// Whether the streak leans warm-gold (for hero/highlight surfaces) or
  /// neutral white (for standard cards on a light background).
  final bool goldTint;

  @override
  Widget build(BuildContext context) {
    final Color tint = goldTint ? const Color(0xFFFFF4CC) : Colors.white;
    return IgnorePointer(
      child: ClipRRect(
        borderRadius: BorderRadius.circular(0),
        child: Opacity(
          opacity: opacity,
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                stops: const [0.0, 0.28, 0.42, 1.0],
                colors: [
                  tint.withValues(alpha: 0.9),
                  tint.withValues(alpha: 0.15),
                  Colors.transparent,
                  Colors.transparent,
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
