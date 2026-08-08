import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import 'widgets/auth_illustration.dart';
import 'widgets/code_boxes.dart';

/// Screen 2 of the login flow: 6-digit OTP verification against the
/// real backend (`/auth/verify-otp`), with a resend countdown that
/// re-requests a fresh OTP from `/auth/send-otp`.
class OtpVerifyPage extends StatefulWidget {
  const OtpVerifyPage({super.key, this.phoneNumber, this.devOtp});

  final String? phoneNumber;

  /// Dev-mode only: the backend currently echoes the generated OTP back
  /// in the send-otp response (no SMS provider wired up yet), so we can
  /// surface it directly for easy testing.
  final String? devOtp;

  @override
  State<OtpVerifyPage> createState() => _OtpVerifyPageState();
}

class _OtpVerifyPageState extends State<OtpVerifyPage> {
  static const int _resendSeconds = 30;
  static const _hasShownHintKey = 'has_shown_demo_otp_hint';

  String _code = '';
  String? _devOtp;
  bool _isVerifying = false;
  bool _hasError = false;
  bool _isResendEnabled = false;
  int _secondsRemaining = _resendSeconds;
  Timer? _timer;

  // Whether the on-screen "Dev mode — OTP: …" hint text should render.
  // The OTP mock itself keeps working identically regardless of this —
  // this only controls whether we print it, and only the very first time
  // a customer ever lands on this screen on this device.
  bool _showHintText = false;

  @override
  void initState() {
    super.initState();
    _devOtp = widget.devOtp;
    _startTimer();
    _initHintVisibility();
  }

  Future<void> _initHintVisibility() async {
    final prefs = await SharedPreferences.getInstance();
    final alreadyShown = prefs.getBool(_hasShownHintKey) ?? false;
    if (!mounted) return;
    if (!alreadyShown) {
      setState(() => _showHintText = true);
      await prefs.setBool(_hasShownHintKey, true);
    }
  }

  void _startTimer() {
    _secondsRemaining = _resendSeconds;
    _isResendEnabled = false;
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (timer) {
      setState(() => _secondsRemaining--);
      if (_secondsRemaining <= 0) {
        setState(() => _isResendEnabled = true);
        timer.cancel();
      }
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _resend() async {
    if (!_isResendEnabled) return;
    final phone = widget.phoneNumber ?? AuthProvider.instance.pendingPhoneNumber;
    if (phone == null) return;
    setState(() {
      _code = '';
      _hasError = false;
    });
    final otp = await AuthProvider.instance.requestOtp(phone);
    if (!mounted) return;
    setState(() => _devOtp = otp);
    _startTimer();
  }

  Future<void> _verify() async {
    if (_code.length != 6) return;
    setState(() {
      _isVerifying = true;
      _hasError = false;
    });
    final success = await AuthProvider.instance.verifyOtp(_code);
    if (!mounted) return;
    setState(() => _isVerifying = false);
    if (!success) {
      setState(() => _hasError = true);
      return;
    }
    context.go('/register');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final phone = widget.phoneNumber ?? AuthProvider.instance.pendingPhoneNumber ?? '';

    return Scaffold(
      appBar: AppBar(),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 8, 24, 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const AuthIllustration(scene: AuthScene.otp),
              const SizedBox(height: 24),
              Text('Verify Mobile Number', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
              const SizedBox(height: 8),
              Text(
                'Enter the 6-digit OTP sent to +91 $phone',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium,
              ),
              if (_devOtp != null && _showHintText) ...[
                const SizedBox(height: 8),
                Text(
                  'Dev mode — OTP: $_devOtp',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.labelMedium?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w700),
                ),
              ],
              const SizedBox(height: 32),
              CodeBoxes(
                autofocus: true,
                hasError: _hasError,
                onChanged: (v) => setState(() {
                  _code = v;
                  _hasError = false;
                }),
                onCompleted: (v) => setState(() => _code = v),
              ),
              if (_hasError) ...[
                const SizedBox(height: 10),
                Text('Incorrect code. Please try again.', style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
              ],
              const SizedBox(height: 24),
              Center(
                child: Column(
                  children: [
                    Text("Didn't receive the code?", style: theme.textTheme.bodySmall),
                    const SizedBox(height: 6),
                    if (_isResendEnabled)
                      GestureDetector(
                        onTap: _resend,
                        child: Text('Resend OTP', style: theme.textTheme.titleSmall?.copyWith(color: AppColors.primary)),
                      )
                    else
                      Text(
                        '00:${_secondsRemaining.toString().padLeft(2, '0')}',
                        style: theme.textTheme.titleSmall,
                      ),
                  ],
                ),
              ),
              const SizedBox(height: 32),
              ElevatedButton(
                onPressed: _code.length == 6 && !_isVerifying ? _verify : null,
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: _isVerifying
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                      )
                    : const Text('Continue'),
              ),
              const SizedBox(height: 10),
              OutlinedButton(
                onPressed: () => context.pop(),
                style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: const Text('Change Mobile Number'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
