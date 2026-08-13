import '../../../../core/error/result.dart';
import '../entities/employee_entity.dart';

abstract class AuthRepository {
  Future<Result<EmployeeEntity>> login({required String name, required String password, required String module});

  Future<Result<EmployeeEntity>> loginWithBiometrics();

  Future<Result<void>> register({required String name, required String password, required String role, required String branchCity});

  Future<Result<void>> forgotPassword({required String identifier});

  Future<Result<void>> logout();

  Future<EmployeeEntity?> currentSession();

  Future<bool> get hasSession;
}
