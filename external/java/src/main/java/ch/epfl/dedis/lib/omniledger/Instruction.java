package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.darc.Identity;
import ch.epfl.dedis.lib.darc.Request;
import ch.epfl.dedis.lib.darc.Signature;
import ch.epfl.dedis.proto.TransactionProto;
import com.google.protobuf.ByteString;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

public class Instruction {
    private ObjectID objId;
    private byte[] nonce;
    private int index;
    private int length;
    private Spawn spawn;
    private Invoke invoke;
    private Delete delete;
    private List<Signature> signatures;

    public Instruction(ObjectID objId, byte[] nonce, int index, int length, Spawn spawn) {
        this.objId = objId;
        this.nonce = nonce;
        this.index = index;
        this.length = length;
        this.spawn = spawn;
    }

    public Instruction(ObjectID objId, byte[] nonce, int index, int length, Invoke invoke) {
        this.objId = objId;
        this.nonce = nonce;
        this.index = index;
        this.length = length;
        this.invoke = invoke;
    }

    public Instruction(ObjectID objId, byte[] nonce, int index, int length, Delete delete) {
        this.objId = objId;
        this.nonce = nonce;
        this.index = index;
        this.length = length;
        this.delete = delete;
    }

    public ObjectID getObjId() {
        return objId;
    }

    public void setSignatures(List<Signature> signatures) {
        this.signatures = signatures;
    }

    public byte[] hash() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            digest.update(this.objId.getDarcId().getId());
            digest.update(this.objId.getInstanceId());
            digest.update(this.nonce);
            digest.update(intToArr(this.index));
            digest.update(intToArr(this.length));
            List<Argument> args= new ArrayList<>();
            if (this.spawn != null) {
                digest.update((byte)(0));
                digest.update(this.spawn.getContractId().getBytes());
                args = this.spawn.getArguments();
            } else if (this.invoke != null) {
                digest.update((byte)(1));
                args = this.invoke.getArguments();
            } else if (this.delete != null) {
                digest.update((byte)(2));
            }
            for (Argument a : args) {
                digest.update(a.getName().getBytes());
                digest.update(a.getValue());
            }
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    private static byte[] intToArr(int x) {
        ByteBuffer b = ByteBuffer.allocate(4);
        b.order(ByteOrder.LITTLE_ENDIAN);
        b.putInt(x);
        return b.array();
    }

    public TransactionProto.Instruction toProto() {
        TransactionProto.Instruction.Builder b = TransactionProto.Instruction.newBuilder();
        b.setObjectid(this.objId.toProto());
        b.setNonce(ByteString.copyFrom(this.nonce));
        b.setIndex(this.index);
        b.setLength(this.length);
        if (this.spawn != null) {
            b.setSpawn(this.spawn.toProto());
        } else if (this.invoke != null) {
            b.setInvoke(this.invoke.toProto());
        } else if (this.delete != null) {
            b.setDelete(this.delete.toProto());
        }
        for (Signature s : this.signatures) {
            b.addSignatures(s.toProto());
        }
        return b.build();
    }

    public String action() {
        String a = "invalid";
        if (this.spawn != null ) {
            a = "Spawn_" + this.spawn.getContractId();
        } else if (this.invoke != null) {
            a = "Invoke_" + this.invoke.getCommand();
        } else if (this.delete != null) {
            a = "Delete";
        }
        return a;
    }

    public Request toDarcRequest() {
        List<Identity> ids = new ArrayList<>();
        List<byte[]> sigs = new ArrayList<>();
        for (Signature sig : this.signatures) {
            ids.add(sig.signer);
            sigs.add(sig.signature);
        }
        return new Request(this.objId.getDarcId(), this.action(), this.hash(), ids, sigs);
    }
}
