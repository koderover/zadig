def increase(x):
    return x + 1

def test_increase_1_to_2021_is_2022():
    assert increase(2021) == 2022

def square(x):
    return x * x

def test_the_square_of_5_is_25():
    assert square(5) == 25

def test_the_square_of_negative_5_is_negative_25():
    assert square(-5) == -25
