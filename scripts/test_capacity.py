import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from capacity import capacity_from_levels  # noqa: E402


class CapacityFromLevels(unittest.TestCase):
    def test_last_level_under_slo(self):
        levels = [{"rps": 10, "p95_ms": 60, "failed_rate": 0}, {"rps": 20, "p95_ms": 120, "failed_rate": 0},
                  {"rps": 30, "p95_ms": 240, "failed_rate": 0}]
        self.assertEqual(capacity_from_levels(levels, slo_ms=200), 20)

    def test_failures_disqualify_a_level(self):
        levels = [{"rps": 10, "p95_ms": 60, "failed_rate": 0}, {"rps": 20, "p95_ms": 90, "failed_rate": 0.02}]
        self.assertEqual(capacity_from_levels(levels, slo_ms=200), 10)

    def test_first_level_already_over_slo_is_an_error(self):
        with self.assertRaises(ValueError):
            capacity_from_levels([{"rps": 10, "p95_ms": 500, "failed_rate": 0}], slo_ms=200)


if __name__ == "__main__":
    unittest.main()
