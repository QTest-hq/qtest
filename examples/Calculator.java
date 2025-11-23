package examples;

/**
 * Simple calculator class for testing
 */
public class Calculator {
    private int lastResult;

    public Calculator() {
        this.lastResult = 0;
    }

    public int add(int a, int b) {
        lastResult = a + b;
        return lastResult;
    }

    public int subtract(int a, int b) {
        lastResult = a - b;
        return lastResult;
    }

    public static int multiply(int a, int b) {
        return a * b;
    }

    private int internalHelper(int x) {
        return x * 2;
    }

    public int getLastResult() {
        return lastResult;
    }
}
