import 'package:flutter/material.dart';

/// A deliberate "reveal" transition (scale + fade, ~300ms) used when
/// jumping straight from a notification tap into a specific loan/letter
/// screen — distinct from the platform's default slide transition so the
/// destination feels like it was surfaced for a reason, not just pushed.
class ScaleFadeRoute<T> extends PageRouteBuilder<T> {
  ScaleFadeRoute({required WidgetBuilder builder})
      : super(
          transitionDuration: const Duration(milliseconds: 300),
          reverseTransitionDuration: const Duration(milliseconds: 220),
          pageBuilder: (context, animation, secondaryAnimation) => builder(context),
          transitionsBuilder: (context, animation, secondaryAnimation, child) {
            final curved = CurvedAnimation(parent: animation, curve: Curves.easeOutCubic);
            return FadeTransition(
              opacity: curved,
              child: ScaleTransition(
                scale: Tween<double>(begin: 0.94, end: 1.0).animate(curved),
                child: child,
              ),
            );
          },
        );
}
