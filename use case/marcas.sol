// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {FunctionsClient} from "https://raw.githubusercontent.com/smartcontractkit/chainlink/v2.19.0/contracts/src/v0.8/functions/v1_0_0/FunctionsClient.sol";
import {FunctionsRequest} from "https://raw.githubusercontent.com/smartcontractkit/chainlink/v2.19.0/contracts/src/v0.8/functions/v1_0_0/libraries/FunctionsRequest.sol";

contract MarcasRegistradas is FunctionsClient {
    using FunctionsRequest for FunctionsRequest.Request;

    bytes32 public donId;
    uint64 public subscriptionId;
    uint32 public gasLimit = 300000;

    string public sourceCode =
        "const nome = args[0];"
        "const top10Nomes = args[1];"
        "const top10Scores = args[2];"
        "const segmento = args[3];"
        "const categoria = args[4];"
        "const apiKey = secrets.geminiKey;"
        "const nomes = top10Nomes.split(',');"
        "const scores = top10Scores.split(',');"
        "const lista = nomes.map((n,i) => `- ${n}: ${scores[i]}%`).join('\\n');"
        "const prompt = ["
        "  'You are Terra Dourada Brands - Brazil LPI 9.279/96. Not legal advice.',"
        "  'Analyze if the requested brand can be registered.',"
        "  '',"
        "  `REQUESTED: ${nome}`,"
        "  `SEGMENT: ${segmento}`,"
        "  `CATEGORY: ${categoria}`,"
        "  '',"
        "  'TOP 10 SIMILAR BRANDS FOUND:',"
        "  lista,"
        "  '',"
        "  'Consider ALL matches above, not only the first one.',"
        "  'If ANY match above 70% exists, tendency is REJECTED.',"
        "  'Consider phonetics, visual and semantic similarity.',"
        "  '',"
        "  'Respond ONLY with one word: APPROVED or REJECTED'"
        "].join('\\n');"
        "const res = await Functions.makeHttpRequest({"
        "  url: `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=${apiKey}`,"
        "  method: 'POST',"
        "  headers: {'Content-Type': 'application/json'},"
        "  data: JSON.stringify({contents: [{parts: [{text: prompt}]}], generationConfig: {temperature: 0.1, maxOutputTokens: 10}})"
        "});"
        "if(res.error) throw new Error('Gemini error');"
        "const text = res.data?.candidates?.[0]?.content?.parts?.[0]?.text || '';"
        "return Functions.encodeUint256(text.trim().toUpperCase().includes('APPROVED') ? 1 : 0);";

    struct Pedido {
        address dono;
        string nome;
        uint8 categoria;
        uint8 score;
        uint256 taxa;
    }
    mapping(bytes32 => Pedido) public pedidos;

    struct Marca {
        string nome;
        uint8 categoria;
        address dono;
        uint256 registradaEm;
        bool ativa;
        uint8 scoreRegistro;
    }

    mapping(bytes32 => Marca) public marcas;
    mapping(uint8 => bytes32[]) public hashsPorCategoria;
    mapping(address => bytes32[]) public marcasDoDono;

    address public owner;
    uint256 public taxaRegistro = 0.001 ether;

    // 1=Farmaceutico 2=Cosmeticos 3=Alimenticio
    // 4=Tecnologia 5=Musica 6=Vestuario 7=Servicos

    event AnaliseSolicitada(bytes32 indexed requestId, string nome, uint8 categoria, string top10);
    event MarcaRegistrada(bytes32 indexed id, string nome, uint8 categoria, address dono, uint256 timestamp, uint8 score);
    event MarcaRejeitada(bytes32 indexed requestId, string nome, string motivo);
    event MarcaInativada(bytes32 indexed id, string nome);
    event MarcaReativada(bytes32 indexed id, string nome);

    constructor(
        address router,
        bytes32 _donId,
        uint64 _subscriptionId
    ) FunctionsClient(router) {
        donId = _donId;
        subscriptionId = _subscriptionId;
        owner = msg.sender;
    }

    function solicitarAnalise(
        string memory nome,
        uint8 categoria,
        uint8 scoreMaior,
        string memory top10Nomes,
        string memory top10Scores,
        string memory segmento,
        bytes memory encryptedSecretsRef
    ) external payable {
        require(categoria >= 1 && categoria <= 7, "Categoria invalida (1-7)");
        require(bytes(nome).length > 0, "Nome vazio");
        require(msg.value >= taxaRegistro, "Taxa insuficiente");

        FunctionsRequest.Request memory req;
        req.initializeRequestForInlineJavaScript(sourceCode);

        string[] memory args = new string[](5);
        args[0] = _toLower(nome);
        args[1] = top10Nomes;
        args[2] = top10Scores;
        args[3] = segmento;
        args[4] = _uint2str(categoria);
        req.setArgs(args);

        if (encryptedSecretsRef.length > 0) {
            req.addSecretsReference(encryptedSecretsRef);
        }

        bytes32 requestId = _sendRequest(req.encodeCBOR(), subscriptionId, gasLimit, donId);

        pedidos[requestId] = Pedido({
            dono: msg.sender,
            nome: _toLower(nome),
            categoria: categoria,
            score: scoreMaior,
            taxa: msg.value
        });

        emit AnaliseSolicitada(requestId, nome, categoria, top10Nomes);
    }

    function fulfillRequest(
        bytes32 requestId,
        bytes memory response,
        bytes memory err
    ) internal override {
        Pedido memory p = pedidos[requestId];
        delete pedidos[requestId];

        if (err.length > 0) {
            payable(p.dono).transfer(p.taxa);
            emit MarcaRejeitada(requestId, p.nome, "Erro no Chainlink");
            return;
        }

        uint256 approved = abi.decode(response, (uint256));

        if (approved == 1) {
            bytes32 id = keccak256(abi.encodePacked(p.nome, p.categoria));

            if (marcas[id].ativa) {
                payable(p.dono).transfer(p.taxa);
                emit MarcaRejeitada(requestId, p.nome, "Marca ja existe");
                return;
            }

            marcas[id] = Marca({
                nome: p.nome,
                categoria: p.categoria,
                dono: p.dono,
                registradaEm: block.timestamp,
                ativa: true,
                scoreRegistro: p.score
            });

            hashsPorCategoria[p.categoria].push(id);
            marcasDoDono[p.dono].push(id);

            emit MarcaRegistrada(id, p.nome, p.categoria, p.dono, block.timestamp, p.score);
        } else {
            payable(p.dono).transfer(p.taxa);
            emit MarcaRejeitada(requestId, p.nome, "Rejeitado pelo Gemini via Chainlink");
        }
    }

    function getNomesAtivos(uint8 categoria) public view returns (string[] memory) {
        bytes32[] storage hashes = hashsPorCategoria[categoria];
        uint256 count = 0;
        for (uint256 i = 0; i < hashes.length; i++) {
            if (marcas[hashes[i]].ativa) count++;
        }
        string[] memory nomes = new string[](count);
        uint256 idx = 0;
        for (uint256 i = 0; i < hashes.length; i++) {
            if (marcas[hashes[i]].ativa) {
                nomes[idx] = marcas[hashes[i]].nome;
                idx++;
            }
        }
        return nomes;
    }

    function verificar(string memory nome, uint8 categoria)
        public view returns (bool ativa, address dono, uint256 registradaEm, uint8 scoreRegistro) {
        bytes32 id = keccak256(abi.encodePacked(_toLower(nome), categoria));
        Marca storage m = marcas[id];
        return (m.ativa, m.dono, m.registradaEm, m.scoreRegistro);
    }

    function gerarId(string memory nome, uint8 categoria) public pure returns (bytes32) {
        return keccak256(abi.encodePacked(_toLower(nome), categoria));
    }

    function inativar(bytes32 id) public {
        Marca storage m = marcas[id];
        require(m.ativa, "Ja inativa");
        require(msg.sender == m.dono || msg.sender == owner, "Sem permissao");
        m.ativa = false;
        emit MarcaInativada(id, m.nome);
    }

    function reativar(bytes32 id) public {
        Marca storage m = marcas[id];
        require(!m.ativa, "Ja ativa");
        require(msg.sender == m.dono || msg.sender == owner, "Sem permissao");
        m.ativa = true;
        emit MarcaReativada(id, m.nome);
    }

    function getMarcasDoDono(address dono) public view returns (bytes32[] memory) {
        return marcasDoDono[dono];
    }

    function setTaxa(uint256 nova) public {
        require(msg.sender == owner, "Sem permissao");
        taxaRegistro = nova;
    }

    function setSubscriptionId(uint64 novo) public {
        require(msg.sender == owner, "Sem permissao");
        subscriptionId = novo;
    }

    function sacar() public {
        require(msg.sender == owner, "Sem permissao");
        payable(owner).transfer(address(this).balance);
    }

    function _toLower(string memory str) internal pure returns (string memory) {
        bytes memory b = bytes(str);
        for (uint i = 0; i < b.length; i++) {
            if (b[i] >= 0x41 && b[i] <= 0x5A) {
                b[i] = bytes1(uint8(b[i]) + 32);
            }
        }
        return string(b);
    }

    function _uint2str(uint256 v) internal pure returns (string memory) {
        if (v == 0) return "0";
        uint256 tmp = v;
        uint256 digits;
        while (tmp != 0) { digits++; tmp /= 10; }
        bytes memory buf = new bytes(digits);
        while (v != 0) { digits--; buf[digits] = bytes1(uint8(48 + v % 10)); v /= 10; }
        return string(buf);
    }

    receive() external payable {}
}
