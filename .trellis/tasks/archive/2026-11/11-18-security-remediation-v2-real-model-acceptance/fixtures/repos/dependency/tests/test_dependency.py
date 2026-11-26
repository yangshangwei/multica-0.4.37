"""Benign contracts for the fictional dependency advisory."""

import unittest

from application import render_label


class DependencyContractTests(unittest.TestCase):
    def test_regular_label_is_trimmed(self) -> None:
        self.assertEqual(render_label("  hello  "), "hello")

    def test_label_at_limit_is_accepted(self) -> None:
        self.assertEqual(render_label("abcdefgh"), "abcdefgh")

    def test_label_over_limit_is_rejected(self) -> None:
        with self.assertRaises(ValueError):
            render_label("abcdefghi")


if __name__ == "__main__":
    unittest.main()
