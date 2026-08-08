import 'package:hive/hive.dart';
import '../../domain/entities/visit_record.dart';

/// Persists completed visit-verification records locally via Hive so field
/// history survives app restarts even fully offline. Swap for a remote
/// datasource once the Go `/employee/visits` sync endpoint exists — the
/// provider consuming this does not need to change.
class VisitHistoryRepository {
  VisitHistoryRepository(this._box);

  final Box<Map> _box;

  Future<void> save(VisitRecord record) async {
    await _box.put(record.id, record.toMap());
  }

  List<VisitRecord> fetchAll() {
    final records = _box.values.map((m) => VisitRecord.fromMap(m)).toList();
    records.sort((a, b) => b.submittedAt.compareTo(a.submittedAt));
    return records;
  }

  VisitRecord? fetchById(String id) {
    final map = _box.get(id);
    return map == null ? null : VisitRecord.fromMap(map);
  }
}
