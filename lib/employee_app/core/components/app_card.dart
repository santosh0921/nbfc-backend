import 'package:flutter/material.dart';
import '../constants/app_colors.dart';
import '../constants/app_spacing.dart';

/// Card in the L&T-style design language — a soft ambient shadow plus a
/// hairline border for real depth, with a quick press-scale for tactile
/// feedback. `withShadow` layers in a stronger shadow on top for cards
/// that should read as elevated above their neighbors (e.g. a selected
/// state); the base shadow is always present now — a bare hairline border
/// with zero elevation read as flat/unfinished on every screen that uses
/// this card, which is nearly all of them.
class AppCard extends StatefulWidget {
  const AppCard({
    super.key,
    required this.child,
    this.onTap,
    this.padding = const EdgeInsets.all(AppSpacing.md),
    this.withShadow = false,
    this.withBorder = true,
  });

  final Widget child;
  final VoidCallback? onTap;
  final EdgeInsets padding;
  final bool withShadow;
  final bool withBorder;

  @override
  State<AppCard> createState() => _AppCardState();
}

class _AppCardState extends State<AppCard> {
  double _scale = 1;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final card = AnimatedScale(
      scale: _scale,
      duration: const Duration(milliseconds: 110),
      curve: Curves.easeOut,
      child: Container(
        padding: widget.padding,
        decoration: BoxDecoration(
          color: theme.colorScheme.surface,
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          border: widget.withBorder ? Border.all(color: AppColors.border) : null,
          boxShadow: [
            BoxShadow(color: AppColors.shadow, blurRadius: 10, offset: const Offset(0, 3)),
            if (widget.withShadow) BoxShadow(color: AppColors.shadow, blurRadius: 22, offset: const Offset(0, 10)),
          ],
        ),
        child: widget.child,
      ),
    );

    if (widget.onTap == null) return card;

    return GestureDetector(
      onTapDown: (_) => setState(() => _scale = 0.97),
      onTapUp: (_) => setState(() => _scale = 1),
      onTapCancel: () => setState(() => _scale = 1),
      onTap: widget.onTap,
      child: card,
    );
  }
}
