import 'package:flutter/material.dart';
import '../../data/repositories/notifications_repository_http.dart';
import '../../domain/entities/notification_entity.dart';

class NotificationsProvider extends ChangeNotifier {
  NotificationsProvider(this._repository);

  final NotificationsRepositoryHttp _repository;

  bool isLoading = false;
  List<NotificationEntity> items = const [];

  /// Set when [load] fails so the screen can tell "genuinely no
  /// notifications yet" apart from "the request failed" — previously
  /// both looked identical (an empty list) because a thrown exception
  /// here left [isLoading] stuck at true forever with nothing shown.
  String? loadError;

  int get unreadCount => items.where((n) => !n.isRead).length;

  Future<void> load() async {
    isLoading = true;
    loadError = null;
    notifyListeners();
    try {
      items = await _repository.fetchAll();
    } catch (_) {
      loadError = 'Could not load notifications. Pull down to try again.';
    }
    isLoading = false;
    notifyListeners();
  }

  Future<void> markAsRead(String id) async {
    await _repository.markAsRead(id);
    await load();
  }

  Future<void> markAllAsRead() async {
    await _repository.markAllAsRead();
    await load();
  }
}
