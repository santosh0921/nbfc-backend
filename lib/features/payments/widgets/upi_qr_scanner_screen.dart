import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';

/// Real camera-based QR scanner for "Scan & Pay". Any `upi://pay?...`
/// QR is parsed and launched via the device's actual UPI intent — this
/// really does hand off to whatever UPI apps (GPay, PhonePe, Paytm...)
/// are installed on the device, exactly like a real merchant QR would.
class UpiQrScannerScreen extends StatefulWidget {
  const UpiQrScannerScreen({super.key});

  @override
  State<UpiQrScannerScreen> createState() => _UpiQrScannerScreenState();
}

class _UpiQrScannerScreenState extends State<UpiQrScannerScreen> {
  final MobileScannerController _controller = MobileScannerController();
  bool _handled = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Map<String, String> _parseUpiUri(String raw) {
    try {
      final uri = Uri.parse(raw);
      if (uri.scheme != 'upi') return {};
      return uri.queryParameters;
    } catch (_) {
      return {};
    }
  }

  Future<void> _handleDetection(BarcodeCapture capture) async {
    if (_handled) return;
    final raw = capture.barcodes.firstOrNull?.rawValue;
    if (raw == null) return;

    final params = _parseUpiUri(raw);
    if (params.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('That doesn\'t look like a UPI payment QR code.')),
      );
      return;
    }

    setState(() => _handled = true);
    await _controller.stop();

    final payeeVpa = params['pa'] ?? '';
    final payeeName = params['pn'] ?? 'Merchant';
    final amount = params['am'];

    if (!mounted) return;
    Navigator.of(context).pop(
      (vpa: payeeVpa, name: payeeName, amount: amount, uri: raw),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: const Text('Scan UPI QR'),
        actions: [
          IconButton(
            onPressed: () => _controller.toggleTorch(),
            icon: const Icon(Icons.flash_on_rounded),
          ),
        ],
      ),
      body: Stack(
        alignment: Alignment.center,
        children: [
          MobileScanner(controller: _controller, onDetect: _handleDetection),
          IgnorePointer(
            child: Container(
              width: 250,
              height: 250,
              decoration: BoxDecoration(
                border: Border.all(color: AppColors.primary, width: 3),
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
            ),
          ),
          const Positioned(
            bottom: 40,
            left: 24,
            right: 24,
            child: Text(
              'Point your camera at any UPI QR code to pay',
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.white, fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }
}

extension _FirstOrNull<T> on List<T> {
  T? get firstOrNull => isEmpty ? null : first;
}

/// Attempts to hand off to a real installed UPI app via its `upi://pay`
/// intent. Returns true if some app accepted the intent.
Future<bool> launchUpiIntent(String upiUri) async {
  final uri = Uri.parse(upiUri);
  return launchUrl(uri, mode: LaunchMode.externalApplication);
}
