package main

// struct element names can't start with nums...
type Bit struct {
	ID      int
	Name    string
	Enabled bool
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func BitGetAll() ([]Bit, error) {
	return globalState.GetAllBits()
}

//////////////////////////////////////////////////////////////////////////////
// DATABASE

func (s *State) GetAllBits() ([]Bit, error) {
	rows, err := s.db.Query("SELECT id, name FROM bits")
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var bit []Bit
	for rows.Next() {
		var arch Bit
		if err := rows.Scan(&arch.ID, &arch.Name); err != nil {
			return bit, err
		}
		bit = append(bit, arch)
	}
	if err = rows.Err(); err != nil {
		return bit, err
	}
	return bit, nil
}

//////////////////////////////////////////////////////////////////////////////
// HTTP
