package company

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("company not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const companyColumns = `id, name, inn, industry, region, description, group_id, okved_code, okved_name, client_type, revenue, city, products, attributes, created_at`

func (r *Repository) List(ctx context.Context) ([]Company, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+companyColumns+`
		FROM companies
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Company, 0)
	for rows.Next() {
		item, err := scanCompany(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id string) (Company, error) {
	item, err := scanCompany(r.pool.QueryRow(ctx, `
		SELECT `+companyColumns+`
		FROM companies
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, ErrNotFound
	}
	if err != nil {
		return Company{}, err
	}
	return item, nil
}

func (r *Repository) Upsert(ctx context.Context, items []Company) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO companies (
				id, name, inn, industry, region, description, group_id,
				okved_code, okved_name, client_type, revenue, city, products, attributes
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				inn = EXCLUDED.inn,
				industry = EXCLUDED.industry,
				region = EXCLUDED.region,
				description = EXCLUDED.description,
				group_id = EXCLUDED.group_id,
				okved_code = EXCLUDED.okved_code,
				okved_name = EXCLUDED.okved_name,
				client_type = EXCLUDED.client_type,
				revenue = EXCLUDED.revenue,
				city = EXCLUDED.city,
				products = EXCLUDED.products,
				attributes = EXCLUDED.attributes
		`, item.ID, item.Name, item.INN, item.Industry, item.Region, item.Description, item.GroupID,
			item.OKVEDCode, item.OKVEDName, item.ClientType, item.Revenue, item.City,
			marshalJSON(item.Products), marshalJSON(item.Attributes)); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) UpsertPeople(ctx context.Context, links []PersonLink) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, link := range links {
		if _, err := tx.Exec(ctx, `
			INSERT INTO people (id, full_name, role, city, region)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				full_name = EXCLUDED.full_name,
				role = EXCLUDED.role,
				city = EXCLUDED.city,
				region = EXCLUDED.region
		`, link.Person.ID, link.Person.FullName, link.Person.Role, link.Person.City, link.Person.Region); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_people (company_id, person_id, relation)
			SELECT $1, $2, $3
			WHERE EXISTS (SELECT 1 FROM companies WHERE id = $1)
			ON CONFLICT (company_id, person_id) DO UPDATE SET relation = EXCLUDED.relation
		`, link.CompanyID, link.Person.ID, link.Relation); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) UpsertPayments(ctx context.Context, payments []Payment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, payment := range payments {
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (payer_id, beneficiary_id, amount, payment_count)
			SELECT $1, $2, $3, $4
			WHERE EXISTS (SELECT 1 FROM companies WHERE id = $1)
			  AND EXISTS (SELECT 1 FROM companies WHERE id = $2)
			ON CONFLICT (payer_id, beneficiary_id) DO UPDATE SET
				amount = EXCLUDED.amount,
				payment_count = EXCLUDED.payment_count
		`, payment.PayerID, payment.BeneficiaryID, payment.Amount, payment.Count); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetWithPeople(ctx context.Context, id string) (CompanyDetails, error) {
	item, err := r.Get(ctx, id)
	if err != nil {
		return CompanyDetails{}, err
	}
	people, err := r.listPeople(ctx, id)
	if err != nil {
		return CompanyDetails{}, err
	}
	relations, err := r.listRelations(ctx, id)
	if err != nil {
		return CompanyDetails{}, err
	}
	return CompanyDetails{Company: item, People: people, Relations: relations}, nil
}

func (r *Repository) ListPeople(ctx context.Context, companyID string) ([]CompanyPerson, error) {
	if _, err := r.Get(ctx, companyID); err != nil {
		return nil, err
	}
	return r.listPeople(ctx, companyID)
}

func (r *Repository) ListRelations(ctx context.Context, companyID string) ([]CompanyRelation, error) {
	if _, err := r.Get(ctx, companyID); err != nil {
		return nil, err
	}
	return r.listRelations(ctx, companyID)
}

func (r *Repository) listPeople(ctx context.Context, companyID string) ([]CompanyPerson, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.full_name, p.role, p.city, p.region, p.created_at, cp.relation
		FROM company_people cp
		JOIN people p ON p.id = cp.person_id
		WHERE cp.company_id = $1
		ORDER BY p.full_name
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CompanyPerson, 0)
	for rows.Next() {
		var item CompanyPerson
		if err := rows.Scan(&item.Person.ID, &item.Person.FullName, &item.Person.Role, &item.Person.City, &item.Person.Region, &item.Person.CreatedAt, &item.Relation); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) listRelations(ctx context.Context, companyID string) ([]CompanyRelation, error) {
	items := make([]CompanyRelation, 0)

	groupRows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name
		FROM companies c
		WHERE c.group_id <> ''
		  AND c.group_id = (SELECT group_id FROM companies WHERE id = $1)
		  AND c.id <> $1
		ORDER BY c.name
	`, companyID)
	if err != nil {
		return nil, err
	}
	for groupRows.Next() {
		item := CompanyRelation{Type: "group", Relation: "same_group"}
		if err := groupRows.Scan(&item.CompanyID, &item.CompanyName); err != nil {
			groupRows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := groupRows.Err(); err != nil {
		groupRows.Close()
		return nil, err
	}
	groupRows.Close()

	peopleRows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, COUNT(DISTINCT cp.person_id)
		FROM company_people cp
		JOIN company_people own ON own.person_id = cp.person_id AND own.company_id = $1
		JOIN companies c ON c.id = cp.company_id
		WHERE cp.company_id <> $1
		GROUP BY c.id, c.name
		ORDER BY c.name
	`, companyID)
	if err != nil {
		return nil, err
	}
	for peopleRows.Next() {
		item := CompanyRelation{Type: "shared_person", Relation: "shared_person"}
		if err := peopleRows.Scan(&item.CompanyID, &item.CompanyName, &item.SharedPeople); err != nil {
			peopleRows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := peopleRows.Err(); err != nil {
		peopleRows.Close()
		return nil, err
	}
	peopleRows.Close()

	paymentRows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, p.payer_id = $1, p.amount::float8, p.payment_count
		FROM payments p
		JOIN companies c ON c.id = CASE WHEN p.payer_id = $1 THEN p.beneficiary_id ELSE p.payer_id END
		WHERE p.payer_id = $1 OR p.beneficiary_id = $1
		ORDER BY c.name
	`, companyID)
	if err != nil {
		return nil, err
	}
	for paymentRows.Next() {
		item := CompanyRelation{Type: "counterparty", Relation: "payment"}
		var outbound bool
		if err := paymentRows.Scan(&item.CompanyID, &item.CompanyName, &outbound, &item.Amount, &item.Count); err != nil {
			paymentRows.Close()
			return nil, err
		}
		if outbound {
			item.Direction = "outbound"
		} else {
			item.Direction = "inbound"
		}
		items = append(items, item)
	}
	if err := paymentRows.Err(); err != nil {
		paymentRows.Close()
		return nil, err
	}
	paymentRows.Close()

	return items, nil
}

func scanCompany(row interface{ Scan(dest ...any) error }) (Company, error) {
	var (
		item       Company
		products   []byte
		attributes []byte
	)
	if err := row.Scan(&item.ID, &item.Name, &item.INN, &item.Industry, &item.Region, &item.Description,
		&item.GroupID, &item.OKVEDCode, &item.OKVEDName, &item.ClientType, &item.Revenue, &item.City,
		&products, &attributes, &item.CreatedAt); err != nil {
		return Company{}, err
	}
	if len(products) > 0 {
		if err := json.Unmarshal(products, &item.Products); err != nil {
			return Company{}, err
		}
	}
	if len(attributes) > 0 {
		if err := json.Unmarshal(attributes, &item.Attributes); err != nil {
			return Company{}, err
		}
	}
	return item, nil
}

func marshalJSON(values map[string]string) []byte {
	if len(values) == 0 {
		return []byte("{}")
	}
	data, err := json.Marshal(values)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
