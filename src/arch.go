package main

type Arch struct {
	ID      int
	Name    string
	Enabled bool
}

//////////////////////////////////////////////////////////////////////////////
// GENERAL PURPOSE

func ArchGetAll() ([]Arch, error) {
	return globalState.GetAllArchs()
}

//////////////////////////////////////////////////////////////////////////////
// DATABASE

func (s *State) GetAllArchs() ([]Arch, error) {
	rows, err := s.db.Query("SELECT id, name FROM archs")
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var archs []Arch
	for rows.Next() {
		var arch Arch
		if err := rows.Scan(&arch.ID, &arch.Name); err != nil {
			return archs, err
		}
		archs = append(archs, arch)
	}
	if err = rows.Err(); err != nil {
		return archs, err
	}
	return archs, nil
}

//////////////////////////////////////////////////////////////////////////////
// HTTP
