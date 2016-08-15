#/bin/bash

read -r -d '' SQL <<EOF
create table db01.t0(
	id int primary key,
	id2 numeric(9, 4) unique not null,
	name varchar(32),
	gender enum('male', 'female'),
	index idx_id_name(id, name(10)),
	foreign key fk_id(id) references t1(id)
) engine=innodb charset=utf8 comment='some comment'"
EOF

go install && echo "$SQL" | sqlfmt
