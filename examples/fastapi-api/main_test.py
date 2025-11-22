import pytest

from main import health_check, list_users



class TestHealthCheck:
    """Tests for HealthCheck"""

    def test_function_returns_expected_status_dictionary(self):
        """Function returns expected status dictionary"""
        # Arrange

        # Act
        result = health_check()

        # Assert
        assert result == {"status": "ok"}


    def test_function_handles_null_input_gracefully(self):
        """Function handles null input gracefully"""
        # Arrange

        # Act
        result = health_check()

        # Assert
        assert result == {"status": "ok"}



class TestListUsers:
    """Tests for ListUsers"""

    def test_returns_all_users_when_there_are_multiple_users_in_the_database(self):
        """Returns all users when there are multiple users in the database"""
        # Arrange
        users_db = [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]

        # Act
        result = list_users(users_db)

        # Assert
        assert result == [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]


    def test_returns_an_empty_list_when_there_are_no_users_in_the_database(self):
        """Returns an empty list when there are no users in the database"""
        # Arrange
        users_db = []

        # Act
        result = list_users(users_db)

        # Assert
        assert result == []


    def test_returns_an_empty_list_when_the_database_is_null(self):
        """Returns an empty list when the database is null"""
        # Arrange
        users_db = None

        # Act
        result = list_users(users_db)

        # Assert
        assert result == []



