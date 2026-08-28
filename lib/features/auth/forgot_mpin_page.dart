import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/auth_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/app_snackbar.dart';
import 'widgets/auth_illustration.dart';
import 'widgets/code_boxes.dart';

enum _Step { otp, newMpin }

/// Forgot-MPIN flow for a device that already has a saved account
/// ([MpinUnlockPage]'s "Forgot MPIN?" link): re-verify the mobile number
/// via the same real OTP endpoints as sign-up, then set a new MPIN through
/// `POST /auth/forgot-mpin`. No shortcut around server-side verification —
/// this is the same OTP + MPIN-hash update the backend already enforces
/// for every other MPIN path.
class ForgotMpinPage extends StatefulWidget {
  const ForgotMpinPage({super.key, required this.mobile});

  final String mobile;

  @override
  State<ForgotMpinPage> createState() => _ForgotMpinPageState();
}

class _ForgotMpinPageState extends State<ForgotMpinPage> {
  static const int _resendSeconds = 30;

  _Step _step = _Step.otp;
  String _otp = '';
  String? _devOtp;
  bool _otpVerified = false;
  bool _isVerifyingOtp = false;
  bool _otpError = false;
  bool _isResendEnabled = false;
  int _secondsRemaining = _resendSeconds;
  Timer? _timer;

  String _newMpin = '';
  String _confirmMpin = '';
  bool _isSubmitting = false;
  String? _mpinError;

  @override
  void initState() {
    super.initState();
    _sendOtp();
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _sendOtp() async {
    final otp = await AuthApiService.sendOtp(widget.mobile);
    if (!mounted) return;
    setState(() => _devOtp = otp);
    _startTimer();
  }

  void _startTimer() {
    _secondsRemaining = _resendSeconds;
    _isResendEnabled = false;
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) return;
      setState(() => _secondsRemaining--);
      if (_secondsRemaining <= 0) {
        setState(() => _isResendEnabled = true);
        timer.cancel();
      }
    });
  }

  Future<void> _resend() async {
    if (!_isResendEnabled) return;
    setState(() {
      _otp = '';
      _otpError = false;
    });
    await _sendOtp();
  }

  Future<void> _verifyOtp() async {
    if (_otp.length != 6) return;
    setState(() {
      _isVerifyingOtp = true;
      _otpError = false;
    });
    try {
      await AuthApiService.verifyOtp(widget.mobile, _otp);
      if (!mounted) return;
      setState(() {
        _isVerifyingOtp = false;
        _otpVerified = true;
        _step = _Step.newMpin;
      });
    } on ApiException {
      if (!mounted) return;
      setState(() {
        _isVerifyingOtp = false;
        _otpError = true;
      });
    }
  }

  Future<void> _submitNewMpin() async {
    if (_newMpin.length != 6 || _confirmMpin.length != 6 || !_otpVerified) return;
    if (_newMpin != _confirmMpin) {
      setState(() => _mpinError = 'New MPIN and confirmation do not match.');
      return;
    }
    setState(() {
      _isSubmitting = true;
      _mpinError = null;
    });
    try {
      await AuthProvider.instance.forgotMpin(widget.mobile, _newMpin, _confirmMpin);
      if (!mounted) return;
      AppSnackBar.showSuccess(context, 'MPIN reset successfully. Sign in with your new MPIN.');
      context.pop();
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _mpinError = e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Reset MPIN')),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 8, 24, 24),
          child: _step == _Step.otp ? _buildOtpStep(theme) : _buildNewMpinStep(theme),
        ),
      ),
    );
  }

  Widget _buildOtpStep(ThemeData theme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const AuthIllustration(scene: AuthScene.otp),
        const SizedBox(height: 24),
        Text('Verify It\'s You', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
        const SizedBox(height: 8),
        Text(
          'Enter the 6-digit OTP sent to +91 ${widget.mobile}',
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium,
        ),
        if (_devOtp != null) ...[
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
          hasError: _otpError,
          onChanged: (v) => setState(() {
            _otp = v;
            _otpError = false;
          }),
          onCompleted: (v) => setState(() => _otp = v),
        ),
        if (_otpError) ...[
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
                Text('00:${_secondsRemaining.toString().padLeft(2, '0')}', style: theme.textTheme.titleSmall),
            ],
          ),
        ),
        const SizedBox(height: 32),
        ElevatedButton(
          onPressed: _otp.length == 6 && !_isVerifyingOtp ? _verifyOtp : null,
          style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
          child: _isVerifyingOtp
              ? const SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                )
              : const Text('Continue'),
        ),
      ],
    );
  }

  Widget _buildNewMpinStep(ThemeData theme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const AuthIllustration(scene: AuthScene.mpin),
        const SizedBox(height: 24),
        Text('Set a New MPIN', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
        const SizedBox(height: 8),
        Text(
          'Choose a 6-digit MPIN you\'ll use to sign in from now on.',
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium,
        ),
        const SizedBox(height: 28),
        Text('New MPIN', style: theme.textTheme.labelMedium),
        const SizedBox(height: 8),
        CodeBoxes(obscure: true, autofocus: true, onChanged: (v) => setState(() {
              _newMpin = v;
              _mpinError = null;
            })),
        const SizedBox(height: 18),
        Text('Confirm New MPIN', style: theme.textTheme.labelMedium),
        const SizedBox(height: 8),
        CodeBoxes(obscure: true, hasError: _mpinError != null, onChanged: (v) => setState(() {
              _confirmMpin = v;
              _mpinError = null;
            })),
        if (_mpinError != null) ...[
          const SizedBox(height: 8),
          Text(_mpinError!, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
        ],
        const SizedBox(height: 24),
        ElevatedButton(
          onPressed: _newMpin.length == 6 && _confirmMpin.length == 6 && !_isSubmitting ? _submitNewMpin : null,
          style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
          child: _isSubmitting
              ? const SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                )
              : const Text('Reset MPIN'),
        ),
      ],
    );
  }
}
