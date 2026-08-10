import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/last_role_storage.dart';
import '../../features/onboarding/onboarding_screen.dart';
import 'animated_background.dart';
import 'glass_logo.dart';
import 'loading_bar.dart';

/// Premium fintech launch experience for Jayashri Capital — a fully
/// custom splash screen: layered moving navy/royal-blue/gold gradient
/// background with ambient glows and floating particles, a breathing
/// glassmorphism logo card, gradient brand wordmark, animated golden
/// divider, and a custom shimmering pill loading bar — all sequenced on
/// a single ~2.7s timeline before handing off to the existing router.
///
/// Two animation controllers do the work:
///  - [_intro] (2700ms, one-shot) sequences every entrance element and
///    fires navigation on completion.
///  - [_ambient] (4000ms, looping) drives the continuous background
///    drift, glow breathing, particle float, and logo float/rotation —
///    kept separate so those loops don't restart or jump when the intro
///    controller finishes.
class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen> with TickerProviderStateMixin {
  static const _totalDuration = Duration(milliseconds: 2700);

  late final AnimationController _intro = AnimationController(vsync: this, duration: _totalDuration);
  late final AnimationController _ambient = AnimationController(vsync: this, duration: const Duration(milliseconds: 4000))..repeat();

  // Timeline fractions, derived from the spec's absolute second marks
  // against the 2.7s total duration.
  late final Animation<double> _logoReveal = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.074, 0.333, curve: Curves.easeOutCubic), // 0.2s -> 0.9s
  );
  late final Animation<double> _brandFade = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.370, 0.481, curve: Curves.easeOut), // 1.0s -> 1.3s
  );
  late final Animation<Offset> _brandSlide = Tween<Offset>(
    begin: const Offset(0, 0.35),
    end: Offset.zero,
  ).animate(CurvedAnimation(parent: _intro, curve: const Interval(0.370, 0.481, curve: Curves.easeOutCubic)));
  late final Animation<double> _taglineFade = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.481, 0.556, curve: Curves.easeOut), // 1.3s -> 1.5s
  );
  late final Animation<Offset> _taglineSlide = Tween<Offset>(
    begin: const Offset(0, 0.25),
    end: Offset.zero,
  ).animate(CurvedAnimation(parent: _intro, curve: const Interval(0.481, 0.556, curve: Curves.easeOutCubic)));
  late final Animation<double> _dividerFade = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.556, 0.593, curve: Curves.easeOut), // 1.5s -> 1.6s
  );
  late final Animation<double> _loadingFill = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.593, 1.0, curve: Curves.easeInOut), // 1.6s -> 2.7s
  );
  late final Animation<double> _bottomTextFade = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.815, 1.0, curve: Curves.easeOut), // 2.2s -> 2.7s
  );
  late final Animation<double> _backgroundFade = CurvedAnimation(
    parent: _intro,
    curve: const Interval(0.0, 0.07, curve: Curves.easeOut),
  );

  @override
  void initState() {
    super.initState();
    _intro.forward();
    _intro.addStatusListener(_onIntroStatusChanged);
  }

  void _onIntroStatusChanged(AnimationStatus status) {
    if (status == AnimationStatus.completed && mounted) {
      _navigateNext();
    }
  }

  Future<void> _navigateNext() async {
    if (AuthProvider.instance.hasSession) {
      context.go('/');
      return;
    }

    // Route back into whichever "side" of the app (customer vs. a specific
    // employee module) was last actively used, instead of always defaulting
    // to the customer login screen. If that side's own session has expired,
    // its own router/login screen takes over from here as normal — this
    // only decides which entry point to land on first.
    final lastRole = await LastRoleStorage.read();
    if (!mounted) return;
    if (lastRole.role == 'employee' && lastRole.module != null) {
      context.go('/employee/${lastRole.module}');
      return;
    }

    // Customer path: a fresh app process has no in-memory session, but if
    // this device already completed the full phone+OTP+MPIN flow before
    // (a token+mobile are still saved), skip straight to the lightweight
    // MPIN-unlock screen instead of forcing that whole flow again. The
    // saved token itself is never trusted blindly — MpinUnlockPage still
    // does a real `/auth/login` call to verify it's still good.
    final savedMobile = await AuthProvider.instance.savedMobileForUnlock();
    if (!mounted) return;
    if (savedMobile != null) {
      context.go('/mpin-unlock');
      return;
    }

    final seenOnboarding = await OnboardingScreen.hasSeenOnboarding();
    if (!mounted) return;
    context.go(seenOnboarding ? '/login' : '/onboarding');
  }

  @override
  void dispose() {
    _intro.removeStatusListener(_onIntroStatusChanged);
    _intro.dispose();
    _ambient.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: SplashPalette.primaryNavy,
      body: AnimatedBuilder(
        animation: Listenable.merge([_intro, _ambient]),
        builder: (context, _) {
          return Stack(
            fit: StackFit.expand,
            children: [
              Opacity(
                opacity: _backgroundFade.value.clamp(0.0, 1.0),
                child: AnimatedBackground(pulse: _ambient.value),
              ),
              SafeArea(
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final isCompact = constraints.maxHeight < 640;
                    return Center(
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
                        child: ConstrainedBox(
                          constraints: BoxConstraints(minHeight: constraints.maxHeight),
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              GlassLogo(reveal: _logoReveal.value, breathe: _ambient.value),
                              SizedBox(height: isCompact ? 24 : 36),
                              FadeTransition(
                                opacity: _brandFade,
                                child: SlideTransition(
                                  position: _brandSlide,
                                  child: _BrandWordmark(compact: isCompact),
                                ),
                              ),
                              const SizedBox(height: 10),
                              FadeTransition(
                                opacity: _taglineFade,
                                child: SlideTransition(
                                  position: _taglineSlide,
                                  child: Text(
                                    'Empowering Financial Growth',
                                    style: TextStyle(
                                      fontSize: 13.5,
                                      color: SplashPalette.pureWhite.withValues(alpha: 0.82),
                                      letterSpacing: 0.6,
                                      fontWeight: FontWeight.w500,
                                    ),
                                  ),
                                ),
                              ),
                              SizedBox(height: isCompact ? 20 : 28),
                              FadeTransition(
                                opacity: _dividerFade,
                                child: const _GoldenDivider(),
                              ),
                              SizedBox(height: isCompact ? 24 : 34),
                              LoadingBar(fillProgress: _loadingFill.value, shimmerProgress: _ambient.value),
                              SizedBox(height: isCompact ? 20 : 28),
                              FadeTransition(
                                opacity: _bottomTextFade,
                                child: Text(
                                  'Secure  •  Trusted  •  Instant',
                                  style: TextStyle(
                                    fontSize: 11.5,
                                    color: SplashPalette.pureWhite.withValues(alpha: 0.55),
                                    letterSpacing: 0.8,
                                    fontWeight: FontWeight.w500,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _BrandWordmark extends StatelessWidget {
  const _BrandWordmark({required this.compact});

  final bool compact;

  @override
  Widget build(BuildContext context) {
    return ShaderMask(
      shaderCallback: (bounds) => const LinearGradient(
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
        colors: [
          SplashPalette.pureWhite,
          SplashPalette.lightGold,
          SplashPalette.premiumGold,
        ],
      ).createShader(bounds),
      child: Text(
        'OneFinance',
        textAlign: TextAlign.center,
        style: TextStyle(
          fontSize: compact ? 30 : 36,
          fontWeight: FontWeight.w900,
          letterSpacing: 0.8,
          color: Colors.white,
          height: 1.1,
        ),
      ),
    );
  }
}

class _GoldenDivider extends StatelessWidget {
  const _GoldenDivider();

  @override
  Widget build(BuildContext context) {
    Widget line() => Container(
          width: 44,
          height: 1,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [
                SplashPalette.premiumGold.withValues(alpha: 0),
                SplashPalette.premiumGold.withValues(alpha: 0.8),
              ],
            ),
          ),
        );

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        line(),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 10),
          child: Icon(Icons.auto_awesome, size: 12, color: SplashPalette.premiumGold.withValues(alpha: 0.9)),
        ),
        Transform.rotate(angle: 3.14159, child: line()),
      ],
    );
  }
}
