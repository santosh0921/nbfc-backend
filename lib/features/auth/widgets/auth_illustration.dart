import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';

/// What the phone in [AuthIllustration] is shown doing — drives the small
/// icon/content inside its screen and the caption glyph beside it.
enum AuthScene { login, otp, mpin, biometric }

/// The framed illustration panel used at the top of Login/OTP/MPIN: a
/// flat, minimal "person beside an oversized phone" scene with two small
/// decorative plant pots, on NBFC's blue/gold palette. Composed entirely
/// from shapes + icons (no image assets), matching the layout rhythm of
/// premium fintech onboarding (illustration card → heading → form →
/// primary CTA) without copying any specific app's artwork or branding.
class AuthIllustration extends StatelessWidget {
  const AuthIllustration({super.key, required this.scene});

  final AuthScene scene;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      height: 232,
      width: double.infinity,
      decoration: BoxDecoration(
        color: isDark ? AppColors.surfaceDark : AppColors.surfaceLight,
        borderRadius: BorderRadius.circular(AppRadius.xl),
        border: Border.all(color: isDark ? AppColors.borderDark : AppColors.borderLight),
        boxShadow: const [BoxShadow(color: Color(0x1410131A), blurRadius: 24, offset: Offset(0, 10))],
      ),
      clipBehavior: Clip.antiAlias,
      child: Stack(
        alignment: Alignment.center,
        children: [
          // Soft radial backdrop tint.
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: RadialGradient(
                  radius: 1.1,
                  colors: [
                    AppColors.primary.withValues(alpha: 0.08),
                    Colors.transparent,
                  ],
                ),
              ),
            ),
          ),
          // Decorative "plant" pots, bottom corners.
          const Positioned(left: 22, bottom: 18, child: _PlantPot()),
          Positioned(right: 24, bottom: 16, child: _PlantPot(flipped: true, tall: scene == AuthScene.mpin)),
          // Person figure.
          const Positioned(left: 8, bottom: 0, child: _PersonFigure()),
          // Oversized phone mockup, center-right.
          _PhoneMockup(scene: scene),
        ],
      ),
    );
  }
}

class _PhoneMockup extends StatelessWidget {
  const _PhoneMockup({required this.scene});

  final AuthScene scene;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 108,
      height: 196,
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 14),
      decoration: BoxDecoration(
        color: AppColors.surfaceLight,
        borderRadius: BorderRadius.circular(22),
        border: Border.all(color: AppColors.textPrimaryLight, width: 2.4),
        boxShadow: [
          BoxShadow(color: AppColors.primary.withValues(alpha: 0.16), blurRadius: 20, offset: const Offset(0, 10)),
        ],
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            width: 30,
            height: 30,
            decoration: BoxDecoration(color: AppColors.primary, shape: BoxShape.circle),
            child: Icon(_iconFor(scene), color: Colors.white, size: 16),
          ),
          const SizedBox(height: 14),
          _ScreenContent(scene: scene),
        ],
      ),
    );
  }

  IconData _iconFor(AuthScene scene) => switch (scene) {
        AuthScene.login => Icons.phone_iphone_rounded,
        AuthScene.otp => Icons.sms_rounded,
        AuthScene.mpin => Icons.lock_rounded,
        AuthScene.biometric => Icons.fingerprint_rounded,
      };
}

class _ScreenContent extends StatelessWidget {
  const _ScreenContent({required this.scene});

  final AuthScene scene;

  @override
  Widget build(BuildContext context) {
    switch (scene) {
      case AuthScene.login:
        return Wrap(
          spacing: 4,
          runSpacing: 4,
          alignment: WrapAlignment.center,
          children: List.generate(
            9,
            (_) => Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(color: AppColors.borderLight, shape: BoxShape.circle),
            ),
          ),
        );
      case AuthScene.otp:
        return Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: List.generate(
            4,
            (_) => Container(
              margin: const EdgeInsets.symmetric(horizontal: 2),
              width: 6,
              height: 6,
              decoration: BoxDecoration(color: AppColors.secondaryDark, shape: BoxShape.circle),
            ),
          ),
        );
      case AuthScene.mpin:
        return Icon(Icons.lock_open_rounded, color: AppColors.secondaryDark, size: 26);
      case AuthScene.biometric:
        return Icon(Icons.fingerprint_rounded, color: AppColors.secondaryDark, size: 30);
    }
  }
}

class _PersonFigure extends StatelessWidget {
  const _PersonFigure();

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 56,
      height: 168,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          Container(
            width: 30,
            height: 30,
            decoration: const BoxDecoration(color: AppColors.secondaryDark, shape: BoxShape.circle),
          ),
          const SizedBox(height: 4),
          Container(
            width: 44,
            height: 78,
            decoration: BoxDecoration(
              color: AppColors.primary,
              borderRadius: BorderRadius.circular(AppRadius.md),
            ),
          ),
          const SizedBox(height: 3),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(width: 16, height: 46, decoration: BoxDecoration(color: AppColors.textPrimaryLight, borderRadius: BorderRadius.circular(8))),
              const SizedBox(width: 6),
              Container(width: 16, height: 46, decoration: BoxDecoration(color: AppColors.textPrimaryLight, borderRadius: BorderRadius.circular(8))),
            ],
          ),
        ],
      ),
    );
  }
}

class _PlantPot extends StatelessWidget {
  const _PlantPot({this.flipped = false, this.tall = false});

  final bool flipped;
  final bool tall;

  @override
  Widget build(BuildContext context) {
    final leaves = Icon(Icons.eco_rounded, color: AppColors.success, size: tall ? 30 : 22);
    return Transform.flip(
      flipX: flipped,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          leaves,
          Container(
            width: 20,
            height: 14,
            decoration: BoxDecoration(
              color: AppColors.secondary,
              borderRadius: const BorderRadius.vertical(bottom: Radius.circular(4)),
            ),
          ),
        ],
      ),
    );
  }
}
