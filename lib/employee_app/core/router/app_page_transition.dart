import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Signature "card-stack" transition: the incoming page rises and scales
/// in from underneath while the outgoing page recedes and dims slightly,
/// giving a layered depth feel. Built from Transform + Opacity only (no
/// filters/shaders) so it stays GPU-cheap and holds 60fps on mid-range
/// hardware.
CustomTransitionPage<T> buildPageWithTransition<T>({
  required BuildContext context,
  required GoRouterState state,
  required Widget child,
}) {
  return CustomTransitionPage<T>(
    key: state.pageKey,
    child: child,
    transitionDuration: const Duration(milliseconds: 380),
    reverseTransitionDuration: const Duration(milliseconds: 320),
    transitionsBuilder: (context, animation, secondaryAnimation, child) {
      final enter = CurvedAnimation(parent: animation, curve: Curves.easeOutCubic, reverseCurve: Curves.easeInCubic);
      final exit = CurvedAnimation(parent: secondaryAnimation, curve: Curves.easeOutCubic, reverseCurve: Curves.easeInCubic);

      final enterScale = Tween(begin: 0.94, end: 1.0).animate(enter);
      final enterSlide = Tween<Offset>(begin: const Offset(0, 0.04), end: Offset.zero).animate(enter);
      final enterFade = Tween(begin: 0.0, end: 1.0).animate(enter);

      final exitScale = Tween(begin: 1.0, end: 0.96).animate(exit);
      final exitFade = Tween(begin: 1.0, end: 0.55).animate(exit);

      return FadeTransition(
        opacity: exitFade,
        child: ScaleTransition(
          scale: exitScale,
          child: FadeTransition(
            opacity: enterFade,
            child: SlideTransition(
              position: enterSlide,
              child: ScaleTransition(scale: enterScale, child: child),
            ),
          ),
        ),
      );
    },
  );
}
