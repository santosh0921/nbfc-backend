import 'package:flutter/material.dart';
import '../../../core/network/api_exception.dart';
import '../../../core/network/video_verification_api_service.dart';
import '../../../core/providers/auth_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/app_snackbar.dart';
import '../../../core/widgets/premium_button.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../core/widgets/premium_text_field.dart';
import '../../../models/video_verification_request.dart';

/// Full-screen "respond to a KYC officer's video verification request".
/// Reached either by tapping the relevant notification directly, or via
/// the banner on the home dashboard — either way it's handed the specific
/// [VideoVerificationRequest] row so it never has to guess which one.
///
/// Once the customer responds (ready or not now), this shows a simple
/// confirmation state and the underlying request is no longer "pending",
/// so neither the notification tap nor the home banner will surface the
/// prompt again for it.
class VideoVerificationRequestPage extends StatefulWidget {
  const VideoVerificationRequestPage({super.key, required this.request});

  final VideoVerificationRequest request;

  @override
  State<VideoVerificationRequestPage> createState() => _VideoVerificationRequestPageState();
}

class _VideoVerificationRequestPageState extends State<VideoVerificationRequestPage> {
  final _controller = TextEditingController();
  bool _submitting = false;
  VideoVerificationRequest? _result;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _respond(bool ready) async {
    final token = AuthProvider.instance.token;
    if (token == null || _submitting) return;
    setState(() => _submitting = true);
    try {
      final updated = await VideoVerificationApiService.respond(
        token,
        widget.request.id,
        ready: ready,
        preferredTime: ready ? _controller.text.trim() : '',
      );
      if (!mounted) return;
      setState(() {
        _result = updated;
        _submitting = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      AppSnackBar.showError(context, e.message);
    } catch (_) {
      if (!mounted) return;
      setState(() => _submitting = false);
      AppSnackBar.showError(context, 'Could not send your response. Please try again.');
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final result = _result;

    return Scaffold(
      appBar: AppBar(title: const Text('Video Verification')),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(20),
          child: result != null ? _buildConfirmation(theme, result) : _buildPrompt(theme),
        ),
      ),
    );
  }

  Widget _buildConfirmation(ThemeData theme, VideoVerificationRequest result) {
    final ready = result.status == 'customer_ready';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 24),
        Icon(
          ready ? Icons.check_circle_rounded : Icons.event_busy_rounded,
          size: 64,
          color: ready ? AppColors.success : AppColors.textSecondaryLight,
        ),
        const SizedBox(height: 20),
        Text(
          ready ? "We've let our team know — they'll call you soon." : "No problem — we'll reach out another time.",
          textAlign: TextAlign.center,
          style: theme.textTheme.titleMedium,
        ),
        if (ready && result.preferredTime.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            'Preferred time: ${result.preferredTime}',
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyMedium,
          ),
        ],
        const SizedBox(height: 24),
        PremiumButton(
          label: 'Done',
          onPressed: () => Navigator.of(context).maybePop(),
        ),
      ],
    );
  }

  Widget _buildPrompt(ThemeData theme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        PremiumCard(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: const Icon(Icons.videocam_rounded, color: AppColors.primary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  'OneFinance would like to verify your KYC documents over a video call.',
                  style: theme.textTheme.bodyLarge,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),
        Text('When are you available?', style: theme.textTheme.titleSmall),
        const SizedBox(height: 8),
        PremiumTextField(
          controller: _controller,
          hint: 'e.g. today 5pm, tomorrow morning',
          enabled: !_submitting,
        ),
        const SizedBox(height: 28),
        PremiumButton(
          label: "I'm Ready",
          icon: Icons.videocam_rounded,
          loading: _submitting,
          onPressed: () => _respond(true),
        ),
        const SizedBox(height: 12),
        PremiumButton(
          label: 'Not Now',
          variant: PremiumButtonVariant.secondary,
          onPressed: _submitting ? null : () => _respond(false),
        ),
      ],
    );
  }
}

/// Small dashboard banner: "Video verification requested — tap to
/// respond." Shown on Home only when there's at least one `pending`
/// request, so the customer doesn't have to dig through Notifications to
/// find it.
class VideoVerificationBanner extends StatelessWidget {
  const VideoVerificationBanner({super.key, required this.request, required this.onTap});

  final VideoVerificationRequest request;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return PremiumCard(
      onTap: onTap,
      radius: AppRadius.lg,
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(12),
            ),
            child: const Icon(Icons.videocam_rounded, color: AppColors.primary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Video verification requested', style: theme.textTheme.titleSmall),
                const SizedBox(height: 2),
                Text('Tap to respond', style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          const Icon(Icons.chevron_right_rounded),
        ],
      ),
    );
  }
}
