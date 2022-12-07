DROP TABLE IF EXISTS mytable;
CREATE TABLE mytable (id serial, name varchar, timestamp timestamp with time zone default NOW());

INSERT INTO mytable(name) values 
('Adrian'),
('Magdalena'),
('Someone');
