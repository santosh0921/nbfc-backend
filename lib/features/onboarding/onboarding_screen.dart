import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:smooth_page_indicator/smooth_page_indicator.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';

class _OnboardPage {
  const _OnboardPage(this.icon, this.title, this.description, this.gradient);
  final IconData icon;
  final String title;
  final String description;
  final List<Color> gradient;
}

/// First-launch, skippable feature-highlight walkthrough shown right
/// after the splash screen — before login. Persists a "seen" flag via
/// SharedPreferences so it only shows once per install.
class OnboardingScreen extends StatefulWidget {
  const OnboardingScreen({super.key});

  static const _prefsKey = 'onboarding_seen';

  static Future<bool> hasSeenOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getBool(_prefsKey) ?? false;
  }

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  final PageController _controller = PageController();
  int _index = 0;

  static const _pages = [
    _OnboardPage(
      Icons.bolt_rounded,
      'Instant Loans, Zero Hassle',
      'Apply for personal, home, gold, and business loans in minutes with a fully digital, paperless process.',
      AppColors.gradientBluePrimary,
    ),
    _OnboardPage(
      Icons.currency_rupee_rounded,
      'Instant Loans for Daily Earners',
      'Need ₹1,000 to ₹50,000 in a hurry? Our Instant Loan is built for daily-wage earners, vendors, and gig workers — minimal paperwork, fast disbursal.',
      AppColors.gradientEmerald,
    ),
    _OnboardPage(
      Icons.fingerprint_rounded,
      'Secure Biometric Access',
      'Log in instantly and safely with fingerprint or face unlock, backed by bank-grade MPIN protection.',
      AppColors.gradientPlum,
    ),
    _OnboardPage(
      Icons.track_changes_rounded,
      'Track Every Step',
      'Follow your application in real time — from submission to disbursal — with live status updates and instant notifications.',
      AppColors.gradientGold,
    ),
  ];

  Future<void> _finish() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(OnboardingScreen._prefsKey, true);
    if (!mounted) return;
    context.go('/login');
  }

  void _next() {
    if (_index == _pages.length - 1) {
      _finish();
      return;
    }
    _controller.nextPage(duration: const Duration(milliseconds: 380), curve: Curves.easeInOutCubic);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isLast = _index == _pages.length - 1;

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            Align(
              alignment: Alignment.topRight,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(0, 8, 8, 0),
                child: TextButton(
                  onPressed: _finish,
                  child: const Text('Skip'),
                ),
              ),
            ),
            Expanded(
              child: PageView.builder(
                controller: _controller,
                itemCount: _pages.length,
                onPageChanged: (i) => setState(() => _index = i),
                itemBuilder: (context, i) {
                  final page = _pages[i];
                  return Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 28),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Container(
                          width: 140,
                          height: 140,
                          decoration: BoxDecoration(
                            gradient: LinearGradient(colors: page.gradient, begin: Alignment.topLeft, end: Alignment.bottomRight),
                            shape: BoxShape.circle,
                            boxShadow: [
                              BoxShadow(color: page.gradient.last.withValues(alpha: 0.35), blurRadius: 30, offset: const Offset(0, 16)),
                            ],
                          ),
                          child: Icon(page.icon, size: 62, color: Colors.white),
                        ),
                        const SizedBox(height: 36),
                        Text(
                          page.title,
                          textAlign: TextAlign.center,
                          style: theme.textTheme.headlineMedium,
                        ),
                        const SizedBox(height: 14),
                        Text(
                          page.description,
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodyMedium,
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
            SmoothPageIndicator(
              controller: _controller,
              count: _pages.length,
              effect: ExpandingDotsEffect(
                dotHeight: 7,
                dotWidth: 7,
                spacing: 7,
                activeDotColor: AppColors.primary,
                dotColor: AppColors.primary.withValues(alpha: 0.18),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(24, 24, 24, 20),
              child: ElevatedButton(
                onPressed: _next,
                style: ElevatedButton.styleFrom(
                  minimumSize: const Size.fromHeight(54),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.md)),
                ),
                child: Text(isLast ? 'Get Started' : 'Next'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
