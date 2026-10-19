import 'dart:async';

import 'package:flutter/material.dart';

import '../network/emi_api_service.dart';
import '../providers/auth_provider.dart';
import '../services/razorpay_checkout_service.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';
import '../../models/emi_schedule.dart';

/// Full-screen payment status — Processing/Success/Failed/Pending, shared
/// by every real-payment entry point in the app (EMI Schedule's Pay Now,
/// the quick Pay EMI screen). This screen NEVER declares the payment
/// successful itself: it only polls GET /auth/payments/:paymentId, which
/// reflects whatever the backend's gateway webhook has already
/// independently verified (internal/payments.RazorpayWebhookHandler).
///
/// It opens the real Razorpay checkout UI on entry whenever the order
/// carries a gatewayKeyId (i.e. a live gateway is actually configured) —
/// the checkout SDK's own success/error callbacks are only ever used for
/// immediate UI feedback (a snackbar), never to mark the payment done.
class GatewayPaymentScreen extends StatefulWidget {
  const GatewayPaymentScreen({
    super.key,
    required this.order,
    required this.amount,
    required this.label,
  });

  final PaymentOrder order;

  /// Rupee amount, for display only — the amount actually charged is
  /// whatever the backend put in the order (widget.order.amountPaise).
  final double amount;

  /// e.g. "Installment #5" or a product name — shown under the amount.
  final String label;

  @override
  State<GatewayPaymentScreen> createState() => _GatewayPaymentScreenState();
}

class _GatewayPaymentScreenState extends State<GatewayPaymentScreen> {
  PaymentStatus? _status;
  Timer? _pollTimer;
  // Wall-clock start time rather than an incrementing "+= 3 every tick"
  // counter — the counter assumed every tick landed exactly 3s apart, so
  // a delayed/dropped tick (backgrounding, a slow response, frame jank)
  // under-counted elapsed time and pushed the real timeout later than
  // intended. A timestamp is exact regardless of how ticks actually land.
  late final DateTime _startedAt = DateTime.now();
  static const _maxWait = Duration(seconds: 180);
  RazorpayCheckoutService? _checkout;

  @override
  void initState() {
    super.initState();
    _startPolling();
    _openCheckoutIfConfigured();
  }

  void _openCheckoutIfConfigured() {
    final keyId = widget.order.gatewayKeyId;
    if (keyId == null || keyId.isEmpty) return;
    _checkout = RazorpayCheckoutService(
      onExternalWallet: (wallet) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Continuing via $wallet…')));
      },
      onError: (message) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
      },
    );
    _checkout!.open(
      keyId: keyId,
      orderId: widget.order.gatewayOrderId,
      amountPaise: widget.order.amountPaise,
      description: widget.label,
    );
  }

  static const _terminalStatuses = {'SUCCESS', 'FAILED', 'CANCELLED', 'REFUNDED', 'PARTIALLY_REFUNDED'};

  void _startPolling() {
    _pollTimer = Timer.periodic(const Duration(seconds: 3), (_) async {
      final token = AuthProvider.instance.token;
      if (token == null) return;
      try {
        final status = await EmiApiService.paymentStatus(token, widget.order.paymentId);
        if (!mounted) return;
        setState(() => _status = status);
        if (_terminalStatuses.contains(status.status)) {
          _pollTimer?.cancel();
        }
      } catch (_) {
        // Transient network error — keep polling silently rather than
        // showing a scary error for what's usually a blip. Still call
        // setState so the screen actually rebuilds and re-evaluates the
        // timeout below — without this, a status fetch that keeps failing
        // (not uncommon on a flaky mobile connection) meant build() never
        // ran again after the very first frame, so the 3-minute timeout
        // fallback could never actually be reached: the screen was stuck
        // on "Processing Payment…" forever no matter how long you waited.
        if (mounted) setState(() {});
      }
      if (mounted && DateTime.now().difference(_startedAt) >= _maxWait) {
        _pollTimer?.cancel();
      }
    });
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _checkout?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final status = _status?.status ?? 'INITIATED';
    final isTerminal = _terminalStatuses.contains(status);
    final timedOut = !isTerminal && DateTime.now().difference(_startedAt) >= _maxWait;

    return PopScope(
      canPop: isTerminal,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) Navigator.of(context).pop(status == 'SUCCESS');
      },
      child: Scaffold(
        backgroundColor: AppColors.primary,
        body: SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(28),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                if (status == 'SUCCESS') ...[
                  const Icon(Icons.check_circle_rounded, color: Colors.white, size: 72),
                  const SizedBox(height: 20),
                  const Text('Payment Successful', style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
                  const SizedBox(height: 8),
                  Text('${widget.label} settled.', style: TextStyle(color: Colors.white.withValues(alpha: 0.85), fontSize: 14)),
                  const SizedBox(height: 8),
                  Text('Reference: ${widget.order.gatewayOrderId}',
                      style: TextStyle(color: Colors.white.withValues(alpha: 0.7), fontSize: 12)),
                ] else if (status == 'FAILED' || status == 'CANCELLED') ...[
                  const Icon(Icons.error_rounded, color: Colors.white, size: 72),
                  const SizedBox(height: 20),
                  const Text('Payment Failed', style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
                  if (_status?.failureReason?.isNotEmpty == true) ...[
                    const SizedBox(height: 8),
                    Text(_status!.failureReason!, textAlign: TextAlign.center, style: TextStyle(color: Colors.white.withValues(alpha: 0.85), fontSize: 14)),
                  ],
                ] else if (timedOut) ...[
                  const Icon(Icons.hourglass_bottom_rounded, color: Colors.white, size: 72),
                  const SizedBox(height: 20),
                  const Text('Payment Pending', style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
                  const SizedBox(height: 8),
                  Text('Still waiting for confirmation — you can safely close this and check back later. We\'ll update your EMI once it\'s confirmed.',
                      textAlign: TextAlign.center, style: TextStyle(color: Colors.white.withValues(alpha: 0.85), fontSize: 14)),
                ] else ...[
                  const SizedBox(
                    width: 56,
                    height: 56,
                    child: CircularProgressIndicator(color: Colors.white, strokeWidth: 3),
                  ),
                  const SizedBox(height: 24),
                  const Text('Processing Payment…', style: TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.w800)),
                  const SizedBox(height: 8),
                  Text('₹${widget.amount.toStringAsFixed(2)} · ${widget.label}',
                      style: TextStyle(color: Colors.white.withValues(alpha: 0.85), fontSize: 14)),
                  const SizedBox(height: 4),
                  Text('Complete the payment in your UPI/banking app. This screen updates automatically — never tap away.',
                      textAlign: TextAlign.center, style: TextStyle(color: Colors.white.withValues(alpha: 0.65), fontSize: 12)),
                ],
                const SizedBox(height: 36),
                if (isTerminal || timedOut)
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: () => Navigator.of(context).pop(status == 'SUCCESS'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: Colors.white,
                        foregroundColor: AppColors.primary,
                        minimumSize: const Size.fromHeight(52),
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.md)),
                      ),
                      child: const Text('Done', style: TextStyle(fontWeight: FontWeight.w700)),
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
