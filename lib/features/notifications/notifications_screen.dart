import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/notification_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/router/scale_fade_route.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/backend_notification.dart';
import '../my_loans/my_loans_screen.dart';

/// Parses which loan (and which letter, if any) a "your letter is ready"
/// notification refers to, from its plain-text title/body — the backend
/// (`internal/loans/admin_handler.go`) doesn't attach a structured
/// `loanId`/`letterType`, only human-readable copy like:
///   "Your Personal Loan application #41 has been sanctioned. ...
///    Please review, sign, and re-upload your Sanction Letter."
///   "Amount Rs.50000.00 for loan #42 disbursed on 2026-08-07. ...
///    Please review, sign, and re-upload your Disbursement Letter."
class LetterNotificationInfo {
  const LetterNotificationInfo({required this.loanId, required this.letterType});

  final int loanId;
  final String letterType; // "Sanction Letter" | "Disbursement Letter"

  /// Returns non-null only for notifications that clearly reference a
  /// ready-to-sign Sanction/Disbursement Letter — the "big, prominent"
  /// notification category this screen highlights.
  static LetterNotificationInfo? parse(BackendNotification n) {
    final text = '${n.title} ${n.body}';
    final idMatch = RegExp(r'#(\d+)').firstMatch(text);
    if (idMatch == null) return null;
    final loanId = int.tryParse(idMatch.group(1)!);
    if (loanId == null) return null;

    if (text.contains('Sanction Letter')) {
      return LetterNotificationInfo(loanId: loanId, letterType: 'Sanction Letter');
    }
    if (text.contains('Disbursement Letter')) {
      return LetterNotificationInfo(loanId: loanId, letterType: 'Disbursement Letter');
    }
    return null;
  }
}

class NotificationsScreen extends StatefulWidget {
  const NotificationsScreen({super.key});

  @override
  State<NotificationsScreen> createState() => _NotificationsScreenState();
}

class _NotificationsScreenState extends State<NotificationsScreen> {
  late Future<List<BackendNotification>> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<List<BackendNotification>> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return [];
    return NotificationApiService.list(token);
  }

  Future<void> _refresh() async {
    setState(() => _future = _load());
    await _future;
  }

  Future<void> _onTapNotification(BackendNotification n) async {
    if (!n.read) {
      final token = AuthProvider.instance.token;
      if (token != null) {
        setState(() => n.read = true);
        try {
          await NotificationApiService.markRead(token, n.id);
        } on ApiException {
          // Non-fatal — the badge will simply reappear on next refresh.
        }
      }
    }

    final letterInfo = LetterNotificationInfo.parse(n);
    if (letterInfo == null || !mounted) return;

    // A deliberate scale+fade "reveal" transition, rather than the
    // platform default slide, straight into the loan's letter section.
    Navigator.of(context, rootNavigator: true).push(
      ScaleFadeRoute(
        builder: (_) => MyLoansScreen(
          initialLoanId: letterInfo.loanId,
          initialLetterType: letterInfo.letterType,
        ),
      ),
    );
  }

  IconData _iconFor(String title) {
    final lower = title.toLowerCase();
    if (lower.contains('sanction')) return Icons.check_circle_rounded;
    if (lower.contains('reject')) return Icons.cancel_rounded;
    if (lower.contains('disburse')) return Icons.account_balance_wallet_rounded;
    if (lower.contains('verif')) return Icons.fact_check_rounded;
    return Icons.notifications_active_rounded;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Notifications')),
      body: SafeArea(
        child: FutureBuilder<List<BackendNotification>>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              final message = snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load notifications.';
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(Icons.error_outline_rounded, size: 48, color: AppColors.error),
                      const SizedBox(height: 12),
                      Text(message, style: theme.textTheme.bodyMedium, textAlign: TextAlign.center),
                      const SizedBox(height: 16),
                      OutlinedButton(onPressed: _refresh, child: const Text('Try Again')),
                    ],
                  ),
                ),
              );
            }
            final notifications = snapshot.data ?? const [];
            if (notifications.isEmpty) {
              return RefreshIndicator(
                onRefresh: _refresh,
                child: ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  children: [
                    SizedBox(
                      height: MediaQuery.of(context).size.height * 0.6,
                      child: Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(Icons.notifications_none_rounded, size: 56, color: AppColors.textSecondaryLight),
                            const SizedBox(height: 12),
                            Text('No notifications yet', style: theme.textTheme.titleMedium),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              );
            }
            return RefreshIndicator(
              onRefresh: _refresh,
              child: ListView.separated(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 40),
                itemCount: notifications.length,
                separatorBuilder: (_, __) => const SizedBox(height: 12),
                itemBuilder: (context, i) {
                  final n = notifications[i];
                  final letterInfo = LetterNotificationInfo.parse(n);
                  if (letterInfo != null) {
                    return _BigLetterNotificationCard(
                      notification: n,
                      onTap: () => _onTapNotification(n),
                    );
                  }
                  return GestureDetector(
                    onTap: () => _onTapNotification(n),
                    child: PremiumCard(
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Container(
                            width: 42,
                            height: 42,
                            decoration: BoxDecoration(
                              color: (n.read ? AppColors.textSecondaryLight : AppColors.primary).withValues(alpha: 0.12),
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: Icon(_iconFor(n.title), size: 20, color: n.read ? AppColors.textSecondaryLight : AppColors.primary),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(n.title, style: theme.textTheme.titleSmall),
                                const SizedBox(height: 2),
                                Text(n.body, style: theme.textTheme.bodySmall),
                                const SizedBox(height: 6),
                                Text(DateFormat('d MMM, h:mm a').format(n.createdAt), style: theme.textTheme.labelSmall),
                              ],
                            ),
                          ),
                          if (!n.read)
                            Container(
                              width: 8,
                              height: 8,
                              margin: const EdgeInsets.only(top: 4),
                              decoration: const BoxDecoration(color: AppColors.primary, shape: BoxShape.circle),
                            ),
                        ],
                      ),
                    ),
                  );
                },
              ),
            );
          },
        ),
      ),
    );
  }
}

/// A visually bigger, more prominent card for "your Sanction/Disbursement
/// Letter is ready to sign" notifications — larger padding, a gradient
/// accent, a document icon, and a one-time fade+slide entrance animation
/// so it stands out from routine notifications in the same list.
class _BigLetterNotificationCard extends StatefulWidget {
  const _BigLetterNotificationCard({required this.notification, required this.onTap});

  final BackendNotification notification;
  final VoidCallback onTap;

  @override
  State<_BigLetterNotificationCard> createState() => _BigLetterNotificationCardState();
}

class _BigLetterNotificationCardState extends State<_BigLetterNotificationCard> with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 450),
  )..forward();
  late final Animation<double> _fade = CurvedAnimation(parent: _controller, curve: Curves.easeOut);
  late final Animation<Offset> _slide = Tween<Offset>(
    begin: const Offset(0, 0.08),
    end: Offset.zero,
  ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic));

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final n = widget.notification;
    return FadeTransition(
      opacity: _fade,
      child: SlideTransition(
        position: _slide,
        child: GestureDetector(
          onTap: widget.onTap,
          child: Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              gradient: const LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: AppColors.gradientBluePrimary,
              ),
              borderRadius: BorderRadius.circular(AppRadius.lg),
              boxShadow: [
                BoxShadow(
                  color: AppColors.primary.withValues(alpha: 0.35),
                  blurRadius: 20,
                  offset: const Offset(0, 10),
                ),
              ],
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 52,
                  height: 52,
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: 0.18),
                    borderRadius: BorderRadius.circular(AppRadius.md),
                  ),
                  child: const Icon(Icons.description_rounded, color: Colors.white, size: 28),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              n.title,
                              style: theme.textTheme.titleMedium?.copyWith(color: Colors.white, fontWeight: FontWeight.w800),
                            ),
                          ),
                          if (!n.read)
                            Container(
                              width: 10,
                              height: 10,
                              decoration: const BoxDecoration(color: AppColors.secondary, shape: BoxShape.circle),
                            ),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Text(
                        n.body,
                        style: theme.textTheme.bodySmall?.copyWith(color: Colors.white.withValues(alpha: 0.92)),
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                            decoration: BoxDecoration(
                              color: Colors.white,
                              borderRadius: BorderRadius.circular(AppRadius.pill),
                            ),
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                const Icon(Icons.draw_rounded, size: 15, color: AppColors.primary),
                                const SizedBox(width: 6),
                                Text(
                                  'eSign Now',
                                  style: theme.textTheme.labelSmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w800),
                                ),
                              ],
                            ),
                          ),
                          const Spacer(),
                          Text(
                            DateFormat('d MMM, h:mm a').format(n.createdAt),
                            style: theme.textTheme.labelSmall?.copyWith(color: Colors.white.withValues(alpha: 0.85)),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
