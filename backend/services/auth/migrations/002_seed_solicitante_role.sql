INSERT INTO roles (nombre, descripcion)
VALUES ('SOLICITANTE', 'Usuario externo que realiza solicitudes mediante Mesa de Partes')
ON CONFLICT (nombre) DO NOTHING;