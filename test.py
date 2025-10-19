from sympy import symbols, solve

# Define the symbols
goal, producedQ, r1, a, T, t0, dr = symbols('goal producedQ r1 a T t0 dr')

# Define the equation
equation = goal - (producedQ + r1 * (a * T - t0) + (r1 + dr) * (1 - a) * T)

# Solve for T
solution = solve(equation, T)

# Print the solution
print(f"The equation solved for T is: T = {solution[0]}")

