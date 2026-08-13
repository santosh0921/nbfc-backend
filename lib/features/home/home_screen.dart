import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/auth_api_service.dart';
import '../../core/network/emi_api_service.dart';
import '../../core/network/notification_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/notification_service.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/dashboard_tour.dart';
import '../../core/widgets/section_header.dart';
import '../../mock/mock_data.dart';
import '../../models/emi_schedule.dart';
import '../../models/loan_product.dart';
import '../notifications/notifications_screen.dart';
import 'widgets/credit_score_card.dart';
import 'widgets/emi_dashboard_summary_card.dart';
import 'widgets/emi_reminder_card.dart';
import 'widgets/financial_tip_card.dart';
import 'widgets/hero_banner_carousel.dart';
import 'widgets/loan_product_card.dart';
import 'widgets/preapproved_card.dart';
import 'widgets/quick_actions_grid.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> with SingleTickerProviderStateMixin {
  late Future<EmiDashboardSummary?> _emiSummaryFuture;

  /// True only when there's at least one unread "big" notification (a
  /// Sanction/Disbursement Letter ready to sign) — the bell only rings
  /// for these, not for every routine unread notification, so it doesn't
  /// feel spammy.
  bool _hasImportantUnread = false;
  bool _hasAnyUnread = false;

  /// The logged-in customer's real name for the header greeting — was
  /// previously always MockData.currentUser's hardcoded "Rajesh Verma"
  /// regardless of who was actually signed in. Null until
  /// [_loadDisplayName] resolves (or if it fails / the profile hasn't
  /// been created yet), in which case the greeting falls back to the mock
  /// name rather than showing nothing.
  String? _displayName;
  String? _avatarInitials;

  late final AnimationController _bellController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 3200),
  );

  /// A repeating "bell shake": a quick ±0.15 rad wiggle (a few short
  /// oscillations), then a pause before it repeats — driven by a single
  /// looping controller so it doesn't feel like a constant vibration.
  late final Animation<double> _bellShake = TweenSequence<double>([
    TweenSequenceItem(tween: Tween(begin: 0.0, end: 0.15), weight: 1),
    TweenSequenceItem(tween: Tween(begin: 0.15, end: -0.15), weight: 1),
    TweenSequenceItem(tween: Tween(begin: -0.15, end: 0.12), weight: 1),
    TweenSequenceItem(tween: Tween(begin: 0.12, end: -0.08), weight: 1),
    TweenSequenceItem(tween: Tween(begin: -0.08, end: 0.0), weight: 1),
    TweenSequenceItem(tween: ConstantTween(0.0), weight: 8),
  ]).animate(CurvedAnimation(parent: _bellController, curve: Curves.easeInOut));

  @override
  void initState() {
    super.initState();
    _emiSummaryFuture = _loadEmiSummary();
    // Auto-plays on every fresh launch (Home's State is only created once
    // per app session — this doesn't re-fire on tab switches).
    WidgetsBinding.instance.addPostFrameCallback((_) => _startTour());
    Future.delayed(const Duration(seconds: 2), _showCongratulationsNotification);
    _loadNotificationState();
    _loadDisplayName();
  }

  /// Best-effort, same pattern as [_loadNotificationState]/[_loadEmiSummary]:
  /// a failure (offline, token expired, profile not created yet) just
  /// means the greeting falls back to the mock name rather than blocking
  /// the dashboard.
  Future<void> _loadDisplayName() async {
    final token = AuthProvider.instance.token;
    if (token == null) return;
    try {
      final res = await AuthApiService.getProfile(token);
      final user = res['user'] as Map<String, dynamic>?;
      final first = (user?['first_name'] as String?)?.trim() ?? '';
      final last = (user?['last_name'] as String?)?.trim() ?? '';
      if (first.isEmpty && last.isEmpty) return;
      final fullName = [first, last].where((s) => s.isNotEmpty).join(' ');
      final initials = [first, last]
          .where((s) => s.isNotEmpty)
          .map((s) => s[0].toUpperCase())
          .join();
      if (!mounted) return;
      setState(() {
        _displayName = fullName;
        _avatarInitials = initials.isEmpty ? null : initials;
      });
    } on ApiException {
      // Non-fatal — greeting falls back to the mock name.
    } catch (_) {
      // Non-fatal.
    }
  }

  @override
  void dispose() {
    _bellController.dispose();
    super.dispose();
  }

  /// Best-effort — mirrors [_loadEmiSummary]: a failure here should never
  /// block the dashboard, it just means the bell doesn't ring this time.
  Future<void> _loadNotificationState() async {
    final token = AuthProvider.instance.token;
    if (token == null) return;
    try {
      final notifications = await NotificationApiService.list(token);
      final hasAnyUnread = notifications.any((n) => !n.read);
      final hasImportantUnread = notifications.any(
        (n) => !n.read && LetterNotificationInfo.parse(n) != null,
      );
      if (!mounted) return;
      setState(() {
        _hasAnyUnread = hasAnyUnread;
        _hasImportantUnread = hasImportantUnread;
      });
      if (hasImportantUnread) {
        _bellController.repeat();
      }
    } on ApiException {
      // Non-fatal — bell simply stays still.
    } catch (_) {
      // Non-fatal.
    }
  }

  /// Best-effort — a failure here (offline, token expired) should never
  /// block the rest of the dashboard from rendering, so it resolves to
  /// null (card simply doesn't show) rather than surfacing an error.
  Future<EmiDashboardSummary?> _loadEmiSummary() async {
    final token = AuthProvider.instance.token;
    if (token == null) return null;
    try {
      return await EmiApiService.dashboardSummary(token);
    } on ApiException {
      return null;
    }
  }

  void _startTour() {
    DashboardTour.start(context, DashboardTourKeys.homeTourSteps());
  }

  void _showCongratulationsNotification() {
    if (!mounted) return;
    final amount = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0).format(MockData.preApprovedAmount);
    NotificationService.instance.show(
      title: 'Congratulations!',
      body: 'You\'re pre-approved for $amount. Apply now with zero extra paperwork.',
    );
  }

  @override
  Widget build(BuildContext context) {
    const user = MockData.currentUser;
    final hour = DateTime.now().hour;
    final greeting = hour < 12 ? 'Good morning' : (hour < 17 ? 'Good afternoon' : 'Good evening');

    final featuredProducts = MockData.loanProducts.take(9).toList();

    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(20, 12, 20, 8),
                child: Row(
                  children: [
                    IconButton(
                      key: DashboardTourKeys.menuButton,
                      onPressed: () => context.push('/menu'),
                      icon: const Icon(Icons.menu_rounded),
                      padding: EdgeInsets.zero,
                      constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
                    ),
                    const SizedBox(width: 6),
                    CircleAvatar(
                      key: DashboardTourKeys.avatarGreeting,
                      radius: 22,
                      backgroundColor: AppColors.primary.withValues(alpha: 0.12),
                      child: Text(
                        _avatarInitials ?? user.avatarInitials,
                        style: const TextStyle(color: AppColors.primary, fontWeight: FontWeight.w800),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(greeting, style: Theme.of(context).textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                          Text(_displayName ?? user.fullName, style: Theme.of(context).textTheme.titleMedium, maxLines: 1, overflow: TextOverflow.ellipsis),
                        ],
                      ),
                    ),
                    IconButton(
                      onPressed: _startTour,
                      icon: const Icon(Icons.help_outline_rounded),
                      tooltip: 'App Walkthrough',
                    ),
                    IconButton(
                      key: DashboardTourKeys.notificationBell,
                      onPressed: () => context.push('/notifications'),
                      icon: AnimatedBuilder(
                        animation: _bellShake,
                        builder: (context, child) => Transform.rotate(
                          angle: _bellShake.value,
                          alignment: Alignment.topCenter,
                          child: child,
                        ),
                        child: Stack(
                          clipBehavior: Clip.none,
                          children: [
                            Icon(_hasImportantUnread ? Icons.notifications_active_rounded : Icons.notifications_none_rounded),
                            if (_hasAnyUnread)
                              Positioned(
                                right: -1,
                                top: -1,
                                child: Container(
                                  width: 9,
                                  height: 9,
                                  decoration: BoxDecoration(
                                    color: _hasImportantUnread ? AppColors.secondaryDark : AppColors.error,
                                    shape: BoxShape.circle,
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 4, 20, 0),
              sliver: SliverToBoxAdapter(
                child: PreApprovedCard(
                  key: DashboardTourKeys.preApprovedCard,
                  amount: MockData.preApprovedAmount,
                  onApply: () => context.push('/quick-apply'),
                ),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 0),
              sliver: SliverToBoxAdapter(
                child: CreditScoreCard(
                  score: MockData.creditScore,
                  band: MockData.creditScoreBand,
                  onTap: () => context.push('/credit-score'),
                ),
              ),
            ),
            if (MockData.activeLoans.isNotEmpty)
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(20, 16, 20, 0),
                sliver: SliverToBoxAdapter(
                  child: EmiReminderCard(
                    loan: MockData.activeLoans.first,
                    onPayNow: () => context.push('/payments'),
                  ),
                ),
              ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 0),
              sliver: SliverToBoxAdapter(
                child: FutureBuilder<EmiDashboardSummary?>(
                  future: _emiSummaryFuture,
                  builder: (context, snapshot) {
                    final summary = snapshot.data;
                    if (summary == null || !summary.hasUpcomingEmi) {
                      return const SizedBox.shrink();
                    }
                    return EmiDashboardSummaryCard(
                      summary: summary,
                      onTap: () => context.push('/emi-schedule', extra: '${summary.loanId}'),
                    );
                  },
                ),
              ),
            ),
            SliverToBoxAdapter(
              child: Padding(
                key: DashboardTourKeys.bannerCarousel,
                padding: const EdgeInsets.fromLTRB(12, 20, 12, 8),
                child: HeroBannerCarousel(banners: MockData.heroBanners),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 0),
              sliver: SliverToBoxAdapter(
                child: Column(
                  key: DashboardTourKeys.exploreLoans,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SectionHeader(
                      title: 'Explore Loans',
                      actionLabel: 'View all',
                      onAction: () => context.go('/loans'),
                    ),
                    const SizedBox(height: 16),
                    GridView.builder(
                      shrinkWrap: true,
                      physics: const NeverScrollableScrollPhysics(),
                      itemCount: featuredProducts.length,
                      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                        crossAxisCount: 3,
                        mainAxisSpacing: 12,
                        crossAxisSpacing: 12,
                        childAspectRatio: 0.92,
                      ),
                      itemBuilder: (context, i) {
                        final LoanProduct product = featuredProducts[i];
                        return LoanProductCard(
                          product: product,
                          onTap: () => context.push('/loans/${product.id}'),
                        );
                      },
                    ),
                  ],
                ),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
              sliver: SliverToBoxAdapter(
                child: Column(
                  key: DashboardTourKeys.quickActions,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Quick Actions', style: Theme.of(context).textTheme.headlineSmall),
                    const SizedBox(height: 12),
                    QuickActionsGrid(
                      actions: [
                        QuickAction(label: 'My Loans', icon: Icons.account_balance_rounded, onTap: () => context.push('/my-loans')),
                        QuickAction(label: 'Pay EMI', icon: Icons.payments_rounded, onTap: () => context.push('/payments')),
                        QuickAction(label: 'EMI Calculator', icon: Icons.calculate_rounded, onTap: () => context.push('/emi-calculator')),
                        QuickAction(label: 'EMI Payments', icon: Icons.event_repeat_rounded, onTap: () => context.go('/emi-payments')),
                        QuickAction(label: 'Support', icon: Icons.support_agent_rounded, onTap: () => context.push('/support')),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 20, 0, 0),
              sliver: SliverToBoxAdapter(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Padding(
                      padding: const EdgeInsets.only(right: 20),
                      child: Text('Financial Tips', style: Theme.of(context).textTheme.headlineSmall),
                    ),
                    const SizedBox(height: 16),
                    SizedBox(
                      height: 168,
                      child: ListView.separated(
                        scrollDirection: Axis.horizontal,
                        itemCount: MockData.financialTips.length,
                        separatorBuilder: (_, __) => const SizedBox(width: 12),
                        itemBuilder: (context, i) => FinancialTipCard(tip: MockData.financialTips[i]),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SliverToBoxAdapter(child: SizedBox(height: 120)),
          ],
        ),
      ),
    );
  }
}
