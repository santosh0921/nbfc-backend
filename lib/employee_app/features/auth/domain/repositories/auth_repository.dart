import '../../../../core/error/result.dart';
import '../entities/employee_entity.dart';

abstract class AuthRepository {
  Future<Result<EmployeeEntity>> login({required String identifier, required String password});

  Future<Result<EmployeeEntity>> loginWithBiometrics();

  Future<Result<void>> forgotPassword({required String identifier});

  Future<Result<void>> logout();

  Future<EmployeeEntity?> currentSession();

  Future<bool> get hasSession;
}
