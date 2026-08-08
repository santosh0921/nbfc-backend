import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

/// First step of the one-time, per-device onboarding sequence shown right
/// after a brand-new Name+Password login on this device. There is no real
/// phone number in this name-based employee login system, so this is a
/// demo/UX-parity touch matching the customer app's OTP step rather than a
/// real security check — the actual security is the name+password already
/// verified by the backend. Entering the fixed demo code just proceeds.
class OtpVerifyPage extends StatefulWidget {
  const OtpVerifyPage({super.key});

  @override
  State<OtpVerifyPage> createState() => _OtpVerifyPageState();
}

class _OtpVerifyPageState extends State<OtpVerifyPage> {
  static const _demoCode = '123456';

  final _formKey = GlobalKey<FormState>();
  final _codeController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  Future<void> _continue() async {
    if (!_formKey.currentState!.validate()) return;
    if (_codeController.text.trim() != _demoCode) {
      AppSnackbar.danger(context, 'Enter the demo verification code shown above.');
      return;
    }
    setState(() => _submitting = true);
    // Purely a UX step — no backend call needed, the OTP is a fixed demo
    // value and the real auth already happened at login.
    await Future<void>.delayed(const Duration(milliseconds: 300));
    if (!mounted) return;
    setState(() => _submitting = false);
    context.pushNamed(RouteNames.mpinSetup);
  }

  /// Back here means "cancel first-time onboarding" — the only coherent
  /// place to send the employee back to is a fresh login, since the
  /// backend session is already active and the router would otherwise just
  /// bounce them straight back to this screen.
  Future<void> _handleBack() async {
    await context.read<AuthProvider>().logout();
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        _handleBack();
      },
      child: AppScaffold(
        showAppBar: false,
        padding: const EdgeInsets.all(AppSpacing.lg),
        body: Center(
          child: SingleChildScrollView(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      width: 72,
                      height: 72,
                      alignment: Alignment.center,
                      decoration: BoxDecoration(
                        color: AppColors.secondary,
                        borderRadius: BorderRadius.circular(AppSpacing.radiusLg),
                      ),
                      child: const Icon(Icons.verified_user_outlined, color: AppColors.primary, size: 34),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    Text('Verify it\'s you', style: Theme.of(context).textTheme.headlineSmall),
                    const SizedBox(height: 4),
                    Text(
                      'One quick step before we set up your account on this device.',
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    Container(
                      width: double.infinity,
                      padding: const EdgeInsets.all(AppSpacing.md),
                      decoration: BoxDecoration(
                        color: AppColors.secondary.withValues(alpha: 0.35),
                        borderRadius: BorderRadius.circular(AppSpacing.radiusMd),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            'Demo verification code',
                            style: Theme.of(context).textTheme.labelMedium?.copyWith(color: AppColors.textSecondary),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            _demoCode,
                            style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                                  letterSpacing: 6,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.primary,
                                ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    AppTextField(
                      label: 'Enter verification code',
                      hint: '6-digit code',
                      controller: _codeController,
                      prefixIcon: Icons.pin_outlined,
                      keyboardType: TextInputType.number,
                      textInputAction: TextInputAction.done,
                      validator: (v) => (v == null || v.trim().length != 6) ? 'Enter the 6-digit code' : null,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    AppButton(label: 'Continue', isLoading: _submitting, onPressed: _continue),
                    const SizedBox(height: AppSpacing.sm),
                    Center(
                      child: TextButton(
                        onPressed: _submitting ? null : _handleBack,
                        child: const Text('Cancel and sign in again'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
