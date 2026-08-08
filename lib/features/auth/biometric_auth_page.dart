import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:local_auth/local_auth.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/notification_service.dart';
import '../../core/services/permission_service.dart';
import 'widgets/auth_illustration.dart';

/// Screen 4 (optional) of the mock login flow: offer to enable device
/// fingerprint/face biometric login. Uses the real device biometric
/// prompt via [LocalAuthentication] where available; skippable, and
/// falls through gracefully (snackbar, stay on screen) if the device has
/// no enrolled biometrics or the user cancels — this is a convenience
/// toggle, not something that blocks getting into the app.
class BiometricAuthPage extends StatefulWidget {
  const BiometricAuthPage({super.key});

  @override
  State<BiometricAuthPage> createState() => _BiometricAuthPageState();
}

class _BiometricAuthPageState extends State<BiometricAuthPage> {
  final LocalAuthentication _localAuth = LocalAuthentication();
  bool _isChecking = false;

  Future<void> _enable() async {
    setState(() => _isChecking = true);
    String? message;
    try {
      final canCheck = await _localAuth.canCheckBiometrics || await _localAuth.isDeviceSupported();
      if (!canCheck) {
        message = 'No biometrics available on this device.';
      } else {
        final authenticated = await _localAuth.authenticate(
          localizedReason: 'Enable fingerprint login for NBFC Premium',
          options: const AuthenticationOptions(biometricOnly: false, stickyAuth: true),
        );
        if (authenticated) {
          if (!mounted) return;
          AuthProvider.instance.setBiometricEnabled(true);
          _finish();
          return;
        }
        message = 'Fingerprint not confirmed. You can try again or skip.';
      }
    } catch (_) {
      message = 'Could not access biometrics on this device.';
    }
    if (!mounted) return;
    setState(() => _isChecking = false);
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  void _finish() {
    PermissionService.requestNotification().then((granted) async {
      if (!granted) return;
      await NotificationService.instance.init();
      NotificationService.instance.show(
        title: 'Welcome to NBFC Premium!',
        body: 'You\'re logged in successfully. Let\'s get you started.',
      );
    });
    context.go('/');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const AuthIllustration(scene: AuthScene.biometric),
              const SizedBox(height: 24),
              Text('Enable Fingerprint Login', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
              const SizedBox(height: 8),
              Text(
                'Quickly and securely access your account using your fingerprint. You can switch to MPIN anytime in settings.',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium,
              ),
              const SizedBox(height: 32),
              ElevatedButton(
                onPressed: _isChecking ? null : _enable,
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: _isChecking
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                      )
                    : const Text('Enable Fingerprint'),
              ),
              const SizedBox(height: 10),
              OutlinedButton(
                onPressed: _finish,
                style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: const Text('Skip for Now'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
