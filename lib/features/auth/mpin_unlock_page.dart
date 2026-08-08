import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/token_storage.dart';
import '../../core/theme/app_colors.dart';
import 'widgets/auth_illustration.dart';
import 'widgets/code_boxes.dart';

/// Lightweight re-entry screen for a device that has already completed
/// the full phone+OTP+MPIN flow once. Shown by [SplashScreen] on a fresh
/// app process when a token+mobile are still saved in [TokenStorage] but
/// the in-memory session hasn't been (re-)established yet. Submits the
/// entered MPIN through the same real `POST /auth/login` call the normal
/// login flow uses — no locally-stored MPIN, no shortcut around
/// server-side verification.
class MpinUnlockPage extends StatefulWidget {
  const MpinUnlockPage({super.key});

  @override
  State<MpinUnlockPage> createState() => _MpinUnlockPageState();
}

class _MpinUnlockPageState extends State<MpinUnlockPage> {
  String _mpin = '';
  String? _maskedMobile;
  bool _isSubmitting = false;
  bool _obscure = true;
  String? _error;

  bool get _canSubmit => _mpin.length == 6;

  @override
  void initState() {
    super.initState();
    _loadMaskedMobile();
  }

  Future<void> _loadMaskedMobile() async {
    final mobile = await TokenStorage.readMobile();
    if (!mounted || mobile == null) return;
    setState(() => _maskedMobile = _mask(mobile));
  }

  String _mask(String mobile) {
    if (mobile.length < 4) return mobile;
    final last4 = mobile.substring(mobile.length - 4);
    return '•••••• $last4';
  }

  Future<void> _submit() async {
    if (!_canSubmit || _isSubmitting) return;
    setState(() {
      _isSubmitting = true;
      _error = null;
    });
    try {
      await AuthProvider.instance.unlockWithMpin(_mpin);
      if (!mounted) return;
      context.go('/');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _mpin = '';
        _error = e.message;
      });
    }
  }

  Future<void> _startOver() async {
    await TokenStorage.clear();
    if (!mounted) return;
    context.go('/login');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    // Splash always routes here via `go` (replacing the stack), so
    // there's nowhere within the app to pop back to — no AppBar/back
    // arrow is shown, and the system back button falls through to its
    // default (exit-app) behavior since the Navigator has nothing left
    // to pop. No PopScope needed.
    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
                const AuthIllustration(scene: AuthScene.mpin),
                const SizedBox(height: 24),
                Text('Welcome Back', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
                const SizedBox(height: 8),
                Text(
                  _maskedMobile != null
                      ? 'Enter your MPIN to continue as $_maskedMobile'
                      : 'Enter your MPIN to continue.',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium,
                ),
                const SizedBox(height: 32),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text('MPIN', style: theme.textTheme.labelMedium),
                    InkWell(
                      onTap: () => setState(() => _obscure = !_obscure),
                      borderRadius: BorderRadius.circular(20),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(_obscure ? Icons.visibility_off_rounded : Icons.visibility_rounded, size: 16, color: AppColors.primary),
                            const SizedBox(width: 4),
                            Text(
                              _obscure ? 'Show' : 'Hide',
                              style: theme.textTheme.labelSmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w700),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                CodeBoxes(
                  obscure: _obscure,
                  autofocus: true,
                  hasError: _error != null,
                  onChanged: (v) => setState(() {
                    _mpin = v;
                    _error = null;
                  }),
                  onCompleted: (v) => setState(() => _mpin = v),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 10),
                  Text(_error!, textAlign: TextAlign.center, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
                ],
                const SizedBox(height: 24),
                ElevatedButton(
                  onPressed: _canSubmit && !_isSubmitting ? _submit : null,
                  style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                  child: _isSubmitting
                      ? const SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                        )
                      : const Text('Unlock'),
                ),
                const SizedBox(height: 16),
                Center(
                  child: TextButton(
                    onPressed: _isSubmitting ? null : _startOver,
                    child: const Text('Use a different account'),
                  ),
                ),
              ],
            ),
          ),
        ),
    );
  }
}
