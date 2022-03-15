CREATE TABLE users(
    id BINARY(16) NOT NULL,
    email VARCHAR(100) NOT NULL,
    emailVerified BIT NOT NULL,
    firstName VARCHAR(100) NOT NULL,
    lastName VARCHAR(100) NOT NULL,
    `password` VARCHAR(255) NOT NULL,
    createdAtUtc DATETIME NOT NULL,
    PRIMARY KEY (id),
    UNIQUE(email)
);

CREATE TABLE entries(
    id BINARY(16) NOT NULL,
    `name` VARCHAR(100) NOT NULL,
    sentByUserId BINARY(16) NOT NULL,
    sentToEmail VARCHAR(100) NOT NULL,
    nonce BINARY(12) NOT NULL,
    `value` VARBINARY(2500) NOT NULL,
    invalidAttempts INT NOT NULL,
    createdAtUtc DATETIME NOT NULL,
    expiresAtUtc DATETIME NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (sentByUserId) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE claimed_entries(
    entryId BINARY(16) NOT NULL,
    `name` VARCHAR(100) NOT NULL,
    sentByUserId BINARY(16) NOT NULL,
    sentToEmail VARCHAR(100) NOT NULL,
    claimedAtUtc DATETIME NOT NULL,
    PRIMARY KEY (entryId),
    FOREIGN KEY (sentByUserId) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE expired_entries(
    entryId BINARY(16) NOT NULL,
    `name` VARCHAR(100) NOT NULL,
    sentByUserId BINARY(16) NOT NULL,
    sentToEmail VARCHAR(100) NOT NULL,
    tooManyAttempts BIT NOT NULL,
    expiredAtUtc DATETIME NOT NULL,
    PRIMARY KEY (entryId),
    FOREIGN KEY (sentByUserId) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE refresh_tokens(
    id BINARY(16) NOT NULL,
    userId BINARY(16) NOT NULL,
    token VARCHAR(50) NOT NULL,
    createdAtUtc DATETIME NOT NULL,
    expiresAtUtc DATETIME NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (userId) REFERENCES users(id) ON DELETE CASCADE
);
