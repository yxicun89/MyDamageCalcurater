ALTER TABLE favorites ADD UNIQUE KEY uq_favorites_device_snapshot (device_id, snapshot_hash);
