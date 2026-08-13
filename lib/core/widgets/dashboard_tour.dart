import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';

/// One stop of the interactive dashboard walkthrough: a real button on
/// screen (found via [targetKey]) gets spotlighted with a cutout in a
/// dark scrim, and a tooltip explains what it does.
class CoachStep {
  const CoachStep({
    required this.targetKey,
    required this.icon,
    required this.title,
    required this.description,
    required this.gradient,
    this.isCircle = false,
  });

  final GlobalKey targetKey;
  final IconData icon;
  final String title;
  final String description;
  final List<Color> gradient;
  final bool isCircle;
}

/// Global keys shared between [HomeScreen]/[AppShell] and whatever
/// launches the tour, so the same physical buttons the user sees are the
/// ones getting spotlighted — not stand-ins.
class DashboardTourKeys {
  DashboardTourKeys._();

  static final menuButton = GlobalKey(debugLabel: 'tour_menu_button');
  static final avatarGreeting = GlobalKey(debugLabel: 'tour_avatar_greeting');
  static final notificationBell = GlobalKey(debugLabel: 'tour_notification_bell');
  static final preApprovedCard = GlobalKey(debugLabel: 'tour_preapproved_card');
  static final bannerCarousel = GlobalKey(debugLabel: 'tour_banner_carousel');
  static final exploreLoans = GlobalKey(debugLabel: 'tour_explore_loans');
  static final quickActions = GlobalKey(debugLabel: 'tour_quick_actions');
  static final fab = GlobalKey(debugLabel: 'tour_fab');
  static final navProfile = GlobalKey(debugLabel: 'tour_nav_profile');

  static List<CoachStep> homeTourSteps() => [
        CoachStep(
          targetKey: menuButton,
          icon: Icons.menu_rounded,
          title: 'Your Command Center',
          description: 'Tap the three lines to open Profile, Documents, Security, Payments, and more.',
          gradient: AppColors.gradientBluePrimary,
        ),
        CoachStep(
          targetKey: avatarGreeting,
          icon: Icons.account_circle_rounded,
          title: 'That\'s You',
          description: 'Your name and profile photo — tap the menu anytime to manage your account.',
          gradient: AppColors.gradientPlum,
        ),
        CoachStep(
          targetKey: notificationBell,
          icon: Icons.notifications_active_rounded,
          title: 'Stay in the Loop',
          description: 'EMI reminders, application updates, and payment confirmations land here — instantly.',
          gradient: AppColors.gradientSunset,
        ),
        CoachStep(
          targetKey: preApprovedCard,
          icon: Icons.workspace_premium_rounded,
          title: 'Pre-Approved, Just for You',
          description: 'See your instant pre-approved loan amount — apply with zero extra paperwork.',
          gradient: AppColors.gradientGold,
        ),
        CoachStep(
          targetKey: bannerCarousel,
          icon: Icons.local_fire_department_rounded,
          title: 'Top Offers',
          description: 'Swipe through featured deals — like our Instant Loan for daily earners, ₹1,000 to ₹50,000.',
          gradient: AppColors.gradientEmerald,
        ),
        CoachStep(
          targetKey: exploreLoans,
          icon: Icons.grid_view_rounded,
          title: 'Every Loan, One Tap Away',
          description: 'Housing, Personal, Gold, Business, and more — browse the full lineup right here.',
          gradient: AppColors.gradientNavy,
        ),
        CoachStep(
          targetKey: quickActions,
          icon: Icons.bolt_rounded,
          title: 'Quick Actions',
          description: 'Jump straight to My Loans, Pay EMI, the EMI Calculator, EMI Payments, or Support.',
          gradient: AppColors.gradientBluePrimary,
        ),
        CoachStep(
          targetKey: fab,
          icon: Icons.rocket_launch_rounded,
          title: 'Quick Apply',
          description: 'Tap this button anytime to start a new loan application in seconds.',
          gradient: AppColors.gradientPlum,
          isCircle: true,
        ),
        CoachStep(
          targetKey: navProfile,
          icon: Icons.shield_rounded,
          title: 'Your Profile',
          description: 'Documents, Security, Language, and app preferences all live in the Profile tab.',
          gradient: AppColors.gradientEmerald,
        ),
      ];
}

/// Shows the coach-mark walkthrough as a full-screen overlay above
/// everything else, driven by [steps] in order. Call [DashboardTour.start]
/// rather than constructing this directly.
class DashboardTour extends StatefulWidget {
  const DashboardTour({super.key, required this.steps, required this.onDone});

  final List<CoachStep> steps;
  final VoidCallback onDone;

  static void start(BuildContext context, List<CoachStep> steps) {
    late OverlayEntry entry;
    entry = OverlayEntry(
      builder: (context) => DashboardTour(
        steps: steps,
        onDone: () => entry.remove(),
      ),
    );
    Overlay.of(context, rootOverlay: true).insert(entry);
  }

  @override
  State<DashboardTour> createState() => _DashboardTourState();
}

class _DashboardTourState extends State<DashboardTour> with TickerProviderStateMixin {
  int _index = 0;
  bool _ready = false;
  Rect? _rect;

  late final AnimationController _pulseController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1400),
  )..repeat(reverse: true);

  late final AnimationController _introController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 260),
  );
  late final Animation<double> _introFade = CurvedAnimation(parent: _introController, curve: Curves.easeOut);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _goToStep(0));
  }

  @override
  void dispose() {
    _pulseController.dispose();
    _introController.dispose();
    super.dispose();
  }

  Rect? _targetRect(GlobalKey key) {
    final box = key.currentContext?.findRenderObject() as RenderBox?;
    if (box == null || !box.attached) return null;
    final topLeft = box.localToGlobal(Offset.zero);
    return topLeft & box.size;
  }

  Future<void> _goToStep(int i) async {
    final ctx = widget.steps[i].targetKey.currentContext;
    if (ctx != null && Scrollable.maybeOf(ctx) != null) {
      try {
        await Scrollable.ensureVisible(ctx, duration: const Duration(milliseconds: 320), curve: Curves.easeInOutCubic, alignment: 0.25);
      } catch (_) {
        // Target isn't inside a scrollable (e.g. the FAB) — nothing to do.
      }
    }
    if (!mounted) return;
    final rect = _targetRect(widget.steps[i].targetKey);
    if (rect == null) {
      // Target isn't currently laid out — skip it instead of showing a
      // broken spotlight.
      if (i >= widget.steps.length - 1) {
        widget.onDone();
      } else {
        _goToStep(i + 1);
      }
      return;
    }
    setState(() {
      _index = i;
      _rect = rect;
      _ready = true;
    });
    _introController.forward(from: 0);
  }

  void _next() {
    if (_index >= widget.steps.length - 1) {
      widget.onDone();
      return;
    }
    _goToStep(_index + 1);
  }

  @override
  Widget build(BuildContext context) {
    if (!_ready || _rect == null) return const SizedBox.shrink();
    final step = widget.steps[_index];
    final screen = MediaQuery.of(context).size;

    final padding = step.isCircle ? 10.0 : 10.0;
    final targetRect = Rect.fromLTRB(
      _rect!.left - padding,
      _rect!.top - padding,
      _rect!.right + padding,
      _rect!.bottom + padding,
    );
    final showTooltipBelow = targetRect.top > screen.height * 0.45;

    return Material(
      color: Colors.transparent,
      child: Stack(
        children: [
          Positioned.fill(
            child: GestureDetector(
              onTap: _next,
              child: TweenAnimationBuilder<Rect?>(
                tween: RectTween(begin: targetRect, end: targetRect),
                duration: const Duration(milliseconds: 380),
                curve: Curves.easeInOutCubic,
                builder: (context, animatedRect, _) {
                  return AnimatedBuilder(
                    animation: _pulseController,
                    builder: (context, __) => CustomPaint(
                      painter: _SpotlightPainter(
                        rect: animatedRect ?? targetRect,
                        isCircle: step.isCircle,
                        accent: step.gradient.last,
                        pulse: _pulseController.value,
                      ),
                      size: Size.infinite,
                    ),
                  );
                },
              ),
            ),
          ),
          AnimatedPositioned(
            duration: const Duration(milliseconds: 380),
            curve: Curves.easeInOutCubic,
            left: 20,
            right: 20,
            top: showTooltipBelow ? null : (targetRect.bottom + 20).clamp(0, screen.height - 220),
            bottom: showTooltipBelow ? (screen.height - targetRect.top + 20) : null,
            child: FadeTransition(
              opacity: _introFade,
              child: SlideTransition(
                position: Tween<Offset>(
                  begin: Offset(0, showTooltipBelow ? 0.06 : -0.06),
                  end: Offset.zero,
                ).animate(_introFade),
                child: _TourTooltip(
                  step: step,
                  index: _index,
                  total: widget.steps.length,
                  onNext: _next,
                  onSkip: widget.onDone,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SpotlightPainter extends CustomPainter {
  _SpotlightPainter({required this.rect, required this.isCircle, required this.accent, required this.pulse});

  final Rect rect;
  final bool isCircle;
  final Color accent;
  final double pulse;

  @override
  void paint(Canvas canvas, Size size) {
    final scrim = Path()..addRect(Rect.fromLTWH(0, 0, size.width, size.height));
    final hole = isCircle
        ? (Path()..addOval(rect))
        : (Path()..addRRect(RRect.fromRectAndRadius(rect, const Radius.circular(AppRadius.md))));
    final combined = Path.combine(PathOperation.difference, scrim, hole);

    canvas.drawPath(
      combined,
      Paint()
        ..shader = LinearGradient(
          colors: [Colors.black.withValues(alpha: 0.80), Colors.black.withValues(alpha: 0.68)],
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
        ).createShader(Rect.fromLTWH(0, 0, size.width, size.height)),
    );

    // Crisp accent ring right on the target.
    canvas.drawPath(
      hole,
      Paint()
        ..color = accent
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2.2,
    );

    // Soft breathing glow ring outside it, driven by [pulse].
    final glowPadding = 4 + pulse * 8;
    final glowRect = rect.inflate(glowPadding);
    final glowPath = isCircle
        ? (Path()..addOval(glowRect))
        : (Path()..addRRect(RRect.fromRectAndRadius(glowRect, Radius.circular(AppRadius.md + glowPadding))));
    canvas.drawPath(
      glowPath,
      Paint()
        ..color = accent.withValues(alpha: (1 - pulse) * 0.45)
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.6,
    );
  }

  @override
  bool shouldRepaint(covariant _SpotlightPainter oldDelegate) =>
      oldDelegate.rect != rect || oldDelegate.isCircle != isCircle || oldDelegate.pulse != pulse || oldDelegate.accent != accent;
}

class _TourTooltip extends StatelessWidget {
  const _TourTooltip({required this.step, required this.index, required this.total, required this.onNext, required this.onSkip});

  final CoachStep step;
  final int index;
  final int total;
  final VoidCallback onNext;
  final VoidCallback onSkip;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final isLast = index == total - 1;
    final progress = (index + 1) / total;

    return Container(
      padding: const EdgeInsets.fromLTRB(20, 18, 20, 18),
      decoration: BoxDecoration(
        color: isDark ? AppColors.surfaceDark : AppColors.surfaceLight,
        borderRadius: BorderRadius.circular(AppRadius.xl),
        border: Border.all(color: step.gradient.last.withValues(alpha: 0.22)),
        boxShadow: [
          BoxShadow(color: step.gradient.last.withValues(alpha: 0.22), blurRadius: 32, offset: const Offset(0, 16)),
          const BoxShadow(color: Colors.black45, blurRadius: 20, offset: Offset(0, 8)),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 42,
                height: 42,
                decoration: BoxDecoration(
                  gradient: LinearGradient(colors: step.gradient, begin: Alignment.topLeft, end: Alignment.bottomRight),
                  borderRadius: BorderRadius.circular(AppRadius.md),
                  boxShadow: [BoxShadow(color: step.gradient.last.withValues(alpha: 0.4), blurRadius: 14, offset: const Offset(0, 6))],
                ),
                child: Icon(step.icon, color: Colors.white, size: 21),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(step.title, style: theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
                    const SizedBox(height: 2),
                    Text(
                      'Step ${index + 1} of $total',
                      style: theme.textTheme.labelSmall?.copyWith(color: step.gradient.last, fontWeight: FontWeight.w700, letterSpacing: 0.3),
                    ),
                  ],
                ),
              ),
              GestureDetector(
                onTap: onSkip,
                child: const Padding(
                  padding: EdgeInsets.all(4),
                  child: Icon(Icons.close_rounded, size: 18, color: AppColors.textSecondaryLight),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text(step.description, style: theme.textTheme.bodySmall?.copyWith(height: 1.4)),
          const SizedBox(height: 16),
          ClipRRect(
            borderRadius: BorderRadius.circular(AppRadius.pill),
            child: TweenAnimationBuilder<double>(
              tween: Tween(begin: 0, end: progress),
              duration: const Duration(milliseconds: 420),
              curve: Curves.easeOutCubic,
              builder: (context, value, _) => LinearProgressIndicator(
                value: value,
                minHeight: 4,
                backgroundColor: step.gradient.last.withValues(alpha: 0.14),
                valueColor: AlwaysStoppedAnimation(step.gradient.last),
              ),
            ),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              TextButton(
                onPressed: onSkip,
                style: TextButton.styleFrom(foregroundColor: AppColors.textSecondaryLight),
                child: const Text('Skip Tour'),
              ),
              const Spacer(),
              SizedBox(
                height: 46,
                child: ElevatedButton.icon(
                  onPressed: onNext,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: step.gradient.last,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(horizontal: 22),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.pill)),
                    elevation: 0,
                  ),
                  icon: Icon(isLast ? Icons.check_rounded : Icons.arrow_forward_rounded, size: 18),
                  label: Text(isLast ? 'Got it' : 'Next', style: const TextStyle(fontWeight: FontWeight.w700)),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
