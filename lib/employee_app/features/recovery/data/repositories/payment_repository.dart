import 'dart:math';
import '../../domain/entities/recovery_case_entity.dart';

/// Repository contract for recovery payment collection — implement against
/// the Go `/employee/recovery/payments` endpoints once they exist.
abstract class PaymentRepository {
  Future<PaymentRecord> recordPayment({
    required String caseId,
    required double amount,
    required PaymentMode mode,
    required String remarks,
  });
  Future<List<PaymentRecord>> fetchPaymentsForCase(String caseId);
}

class PaymentRepositoryMock implements PaymentRepository {
  static final Map<String, List<PaymentRecord>> _payments = {};
  static final _random = Random();

  @override
  Future<PaymentRecord> recordPayment({
    required String caseId,
    required double amount,
    required PaymentMode mode,
    required String remarks,
  }) async {
    await Future.delayed(const Duration(milliseconds: 500));
    final receiptNumber = 'RCT${DateTime.now().millisecondsSinceEpoch % 1000000}${_random.nextInt(90) + 10}';
    final record = PaymentRecord(
      amount: amount,
      mode: mode,
      receiptNumber: receiptNumber,
      remarks: remarks,
      recordedAt: DateTime.now(),
    );
    _payments.putIfAbsent(caseId, () => []).add(record);
    return record;
  }

  @override
  Future<List<PaymentRecord>> fetchPaymentsForCase(String caseId) async {
    await Future.delayed(const Duration(milliseconds: 250));
    return List.unmodifiable(_payments[caseId] ?? const []);
  }
}
