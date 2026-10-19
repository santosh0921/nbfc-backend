import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:local_auth/local_auth.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/token_storage.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/app_snackbar.dart';
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
  final LocalAuthentication _localAuth = LocalAuthentication();

  String _mpin = '';
  String? _maskedMobile;
  bool _isSubmitting = false;
  bool _obscure = true;
  String? _error;
  bool _showFingerprintOption = false;

  bool get _canSubmit => _mpin.length == 6;

  @override
  void initState() {
    super.initState();
    _loadMaskedMobile();
    _checkFingerprintAvailability();
  }

  Future<void> _loadMaskedMobile() async {
    final mobile = await TokenStorage.readMobile();
    if (!mounted || mobile == null) return;
    setState(() => _maskedMobile = _mask(mobile));
  }

  Future<void> _checkFingerprintAvailability() async {
    if (!AuthProvider.instance.biometricEnabled) return;
    bool available = false;
    try {
      available = await _localAuth.canCheckBiometrics || await _localAuth.isDeviceSupported();
    } catch (_) {
      available = false;
    }
    if (!mounted || !available) return;
    setState(() => _showFingerprintOption = true);
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
      await _maybeOfferBiometricEnrollment(_mpin);
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

  Future<void> _submitWithBiometric() async {
    if (_isSubmitting) return;
    setState(() {
      _isSubmitting = true;
      _error = null;
    });
    try {
      final authenticated = await _localAuth.authenticate(
        localizedReason: 'Unlock OneFinance',
        options: const AuthenticationOptions(biometricOnly: false, stickyAuth: true),
      );
      if (!authenticated) {
        if (!mounted) return;
        setState(() => _isSubmitting = false);
        return;
      }
      await AuthProvider.instance.unlockWithBiometric();
      if (!mounted) return;
      context.go('/');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _error = e.message;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _error = 'Could not access biometrics on this device.';
      });
    }
  }

  /// After a manual MPIN unlock succeeds, ask (at most once, after a few
  /// unlocks) whether the customer wants to switch to fingerprint login —
  /// this is the only point in the unlock flow where the MPIN is known in
  /// plaintext to store for biometric use.
  Future<void> _maybeOfferBiometricEnrollment(String mpin) async {
    if (!await AuthProvider.instance.shouldOfferBiometricEnrollment()) return;
    if (!mounted) return;
    final enable = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Use fingerprint to sign in?'),
        content: const Text('Skip typing your MPIN next time — unlock instantly with your fingerprint.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: const Text('Not now')),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: const Text('Enable')),
        ],
      ),
    );
    if (enable == true) {
      try {
        final authenticated = await _localAuth.authenticate(
          localizedReason: 'Enable fingerprint login for OneFinance',
          options: const AuthenticationOptions(biometricOnly: false, stickyAuth: true),
        );
        if (authenticated) {
          await AuthProvider.instance.enableBiometricLogin(mpin);
          if (mounted) AppSnackBar.showSuccess(context, 'Fingerprint login enabled.');
        }
      } catch (_) {
        // Best-effort — the customer can still enable it later from Security.
      }
    } else {
      await AuthProvider.instance.dismissBiometricOffer();
    }
  }

  Future<void> _startOver() async {
    await TokenStorage.clear();
    if (!mounted) return;
    context.go('/login');
  }

  Future<void> _forgotMpin() async {
    final mobile = await TokenStorage.readMobile();
    if (!mounted || mobile == null) return;
    context.push('/forgot-mpin', extra: mobile);
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
                const SizedBox(height: 10),
                Center(
                  child: TextButton(
                    onPressed: _isSubmitting ? null : _forgotMpin,
                    child: const Text('Forgot MPIN?'),
                  ),
                ),
                const SizedBox(height: 14),
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
                if (_showFingerprintOption) ...[
                  const SizedBox(height: 12),
                  OutlinedButton.icon(
                    onPressed: _isSubmitting ? null : _submitWithBiometric,
                    style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                    icon: const Icon(Icons.fingerprint_rounded),
                    label: const Text('Unlock with Fingerprint'),
                  ),
                ],
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
