
import { getAllUsers, getUserById } from './app';



describe('getAllUsers', () => {

  test('Successfully retrieves all users', () => {
    // Arrange
    const req = {  };
    const res = { json: [{ name: 'John', id: 1 }] };

    // Act
    const result = getAllUsers(req, res);

    // Assert
    expect(res.json.call_args[0][0]).toBe([{ id: 1, name: 'John' }]);
  });


  test('Returns empty array when no users are available', () => {
    // Arrange
    const req = {  };
    const res = { json: [] };

    // Act
    const result = getAllUsers(req, res);

    // Assert
    expect(res.json.call_args[0][0]).toBe([]);
  });


  test('Returns single user when only one is available', () => {
    // Arrange
    const req = {  };
    const res = { json: [{ name: 'John', id: 1 }] };

    // Act
    const result = getAllUsers(req, res);

    // Assert
    expect(res.json.call_args[0][0]).toBe([{ id: 1, name: 'John' }]);
  });


  test('Throws error if res is not provided', () => {
    // Arrange
    const req = {  };

    // Act
    const result = getAllUsers(req);

    // Assert
    expect(() => result).toThrow();
  });

});

describe('getUserById', () => {

  test('Retrieving a user with a valid ID returns the correct user object', () => {
    // Arrange
    const req = { params: { id: 123 } };
    const res = {  };

    // Act
    const result = getUserById(req, res);

    // Assert
    expect(res.json.call_args[0][0]).toBe({ id: 123, name: 'John' });
  });


  test('Retrieving a user with ID zero returns an error', () => {
    // Arrange
    const req = { params: { id: 0 } };
    const res = {  };

    // Act
    const result = getUserById(req, res);

    // Assert
    expect(res.status.call_args[0][0]).toBe(400);
  });


  test('Retrieving a user with negative ID returns an error', () => {
    // Arrange
    const req = { params: { id: -1 } };
    const res = {  };

    // Act
    const result = getUserById(req, res);

    // Assert
    expect(res.status.call_args[0][0]).toBe(400);
  });


  test('Retrieving a user with an empty ID returns an error', () => {
    // Arrange
    const req = { params: { id: '' } };
    const res = {  };

    // Act
    const result = getUserById(req, res);

    // Assert
    expect(res.status.call_args[0][0]).toBe(400);
  });


  test('Retrieving a user with null ID returns an error', () => {
    // Arrange
    const req = { params: { id: null } };
    const res = {  };

    // Act
    const result = getUserById(req, res);

    // Assert
    expect(res.status.call_args[0][0]).toBe(400);
  });

});

