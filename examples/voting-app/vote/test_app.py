import app


def test_generate_voter_id():
    assert app.get_voter_id({}), "voter id is generated"


def test_get_voter_id():
    assert app.get_voter_id({"voter_id": "12345"}) == "12345", "voter id is pre-defined" 
